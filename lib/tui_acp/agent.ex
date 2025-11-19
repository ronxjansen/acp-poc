defmodule TuiAcp.Agent do
  @moduledoc """
  An ACP agent that provides weather information using LLM integration.
  """
  @behaviour ACPex.Agent

  require Logger

  @impl true
  def init(args) do
    state = %{
      sessions: %{},
      notification_callback: Keyword.get(args, :notification_callback),
      request_callback: Keyword.get(args, :request_callback)
    }

    {:ok, state}
  end

  @impl true
  def handle_initialize(_request, state) do
    response = %ACPex.Schema.Connection.InitializeResponse{
      protocol_version: 1,
      agent_capabilities: %{
        sessions: %{new: true, load: false},
        prompt_capabilities: %{text: true}
      }
    }

    {:ok, response, state}
  end

  @impl true
  def handle_new_session(_request, state) do
    session_id = generate_session_id()

    system_message =
      "You are a helpful AI assistant with access to various tools. You can:
      - Get weather information using get_current_weather and get_forecast tools
      - Read files from the user's filesystem using the read_file tool
      - Write files to the user's filesystem using the write_file tool
      - Search for text patterns in files using the grep_search tool

      When users ask about weather, use the weather tools. When they ask about files or need to work with the filesystem, use the file tools. When they need to search for text or patterns across files, use the grep_search tool. Always be specific and provide detailed, friendly responses."

    session_state = %{
      id: session_id,
      context: ReqLLM.Context.new([ReqLLM.Context.system(system_message)]),
      created_at: DateTime.utc_now()
    }

    new_state = put_in(state, [:sessions, session_id], session_state)

    response = %ACPex.Schema.Session.NewResponse{
      session_id: session_id
    }

    {:ok, response, new_state}
  end

  @impl true
  def handle_load_session(_request, state) do
    {:error, :not_supported, state}
  end

  @impl true
  def handle_prompt(request, state) do
    session_id = request.session_id
    user_message = extract_user_message(request.prompt)

    Logger.info("Processing prompt for session #{session_id}: #{user_message}")

    current_context = state.sessions[session_id].context
    updated_context = ReqLLM.Context.append(current_context, ReqLLM.Context.user(user_message))

    case run_agent_loop(session_id, updated_context, state, max_turns: 10) do
      {:ok, new_context, assistant_message} ->
        state = put_in(state, [:sessions, session_id, :context], new_context)

        send_text_chunk(session_id, assistant_message, state)

        response = %ACPex.Schema.Session.PromptResponse{
          stop_reason: "done"
        }

        {:ok, response, state}

      {:error, reason} ->
        error_msg = "Error processing request: #{inspect(reason)}"
        send_text_chunk(session_id, error_msg, state)

        response = %ACPex.Schema.Session.PromptResponse{
          stop_reason: "error"
        }

        {:ok, response, state}
    end
  end

  @impl true
  def handle_cancel(_request, state) do
    {:ok, state}
  end

  @impl true
  def handle_authenticate(_request, state) do
    response = %ACPex.Schema.Connection.AuthenticateResponse{
      authenticated: true
    }

    {:ok, response, state}
  end

  defp generate_session_id do
    "session_" <> (:crypto.strong_rand_bytes(16) |> Base.encode16(case: :lower))
  end

  defp extract_user_message(prompt) when is_list(prompt) do
    prompt
    |> Enum.map(fn block ->
      case block do
        %{type: "text", text: text} -> text
        %{"type" => "text", "text" => text} -> text
        _ -> ""
      end
    end)
    |> Enum.join("\n")
  end

  defp extract_user_message(prompt) when is_binary(prompt), do: prompt
  defp extract_user_message(_), do: ""

  defp send_text_chunk(session_id, text, state) do
    update = %{
      type: "agent_message_chunk",
      content: %{
        type: "text",
        text: text
      }
    }

    params = %{
      session_id: session_id,
      update: update
    }

    if state.notification_callback do
      state.notification_callback.("session/update", params)
    else
      send(TuiAcp.Server, {:send_notification, "session/update", params})
    end
  end

  defp run_agent_loop(session_id, context, state, opts) do
    max_turns = Keyword.get(opts, :max_turns, 10)
    run_agent_loop(session_id, context, state, 0, max_turns)
  end

  defp run_agent_loop(_session_id, context, _state, current_turn, max_turns)
       when current_turn >= max_turns do
    content = ReqLLM.Response.text(List.last(context.messages)) || "Max turns reached."
    {:ok, context, content}
  end

  defp run_agent_loop(session_id, context, state, current_turn, max_turns) do
    tools = get_tools()

    case ReqLLM.generate_text("anthropic:claude-haiku-4-5", context,
           tools: tools,
           temperature: 0.7
         ) do
      {:ok, response} ->
        if has_tool_calls?(response.context) do
          case execute_tools_in_context(session_id, response.context, tools, state) do
            {:ok, updated_context} ->
              run_agent_loop(session_id, updated_context, state, current_turn + 1, max_turns)

            {:error, reason} ->
              {:error, reason}
          end
        else
          content =
            ReqLLM.Response.text(response) || "I apologize, but I couldn't generate a response."

          {:ok, response.context, content}
        end

      {:error, reason} ->
        Logger.error("Error calling LLM via ReqLLM: #{inspect(reason)}")
        {:error, reason}
    end
  rescue
    e ->
      Logger.error("Error calling LLM: #{inspect(e)}")
      {:error, e}
  end

  defp get_tools do
    [
      ReqLLM.Tool.new!(
        name: "get_current_weather",
        description: "Get the current weather for a specific location",
        parameter_schema: [
          location: [
            type: :string,
            required: true,
            doc: "The city and state, e.g. San Francisco, CA"
          ],
          unit: [
            type: :string,
            doc: "The temperature unit to use (celsius or fahrenheit)"
          ]
        ],
        callback: {__MODULE__, :execute_weather_tool, []}
      ),
      ReqLLM.Tool.new!(
        name: "get_forecast",
        description: "Get the weather forecast for a specific location",
        parameter_schema: [
          location: [
            type: :string,
            required: true,
            doc: "The city and state, e.g. San Francisco, CA"
          ],
          days: [
            type: :integer,
            required: true,
            doc: "Number of days to forecast (1-7)"
          ],
          unit: [
            type: :string,
            doc: "The temperature unit to use (celsius or fahrenheit)"
          ]
        ],
        callback: {__MODULE__, :execute_forecast_tool, []}
      ),
      ReqLLM.Tool.new!(
        name: "read_file",
        description: "Read the contents of a text file from the filesystem",
        parameter_schema: [
          path: [
            type: :string,
            required: true,
            doc: "The absolute or relative path to the file to read"
          ]
        ],
        callback: {__MODULE__, :execute_read_file_tool, []}
      ),
      ReqLLM.Tool.new!(
        name: "write_file",
        description: "Write content to a text file on the filesystem",
        parameter_schema: [
          path: [
            type: :string,
            required: true,
            doc: "The absolute or relative path to the file to write"
          ],
          content: [
            type: :string,
            required: true,
            doc: "The content to write to the file"
          ]
        ],
        callback: {__MODULE__, :execute_write_file_tool, []}
      ),
      ReqLLM.Tool.new!(
        name: "grep_search",
        description: "Search for a pattern in files using grep-like functionality. Returns matching lines with file paths and line numbers.",
        parameter_schema: [
          pattern: [
            type: :string,
            required: true,
            doc: "The search pattern or text to find in files"
          ],
          path: [
            type: :string,
            doc: "The directory or file path to search in (defaults to current directory)"
          ],
          case_sensitive: [
            type: :boolean,
            doc: "Whether the search should be case-sensitive (defaults to false)"
          ],
          file_pattern: [
            type: :string,
            doc: "Glob pattern to filter files (e.g., '*.ex' for Elixir files, '*.{ex,exs}' for multiple extensions)"
          ]
        ],
        callback: {__MODULE__, :execute_grep_search_tool, []}
      )
    ]
  end

  defp has_tool_calls?(context) do
    case List.last(context.messages) do
      nil ->
        false

      message ->
        is_list(message.tool_calls) and length(message.tool_calls) > 0
    end
  end

  defp execute_tools_in_context(session_id, context, tools, state) do
    latest_message = List.last(context.messages)
    tool_calls = latest_message.tool_calls || []

    case execute_all_tools(session_id, tool_calls, tools, state) do
      {:ok, tool_results} ->
        updated_context = ReqLLM.Context.append(context, tool_results)
        {:ok, updated_context}

      {:error, _reason} = error ->
        error
    end
  end

  defp execute_all_tools(session_id, tool_calls, tools, state) do
    tool_calls
    |> Enum.reduce_while({:ok, []}, fn tool_call, {:ok, results} ->
      case execute_single_tool(session_id, tool_call, tools, state) do
        {:ok, result} ->
          {:cont, {:ok, [result | results]}}

        {:error, _reason} = error ->
          {:halt, error}
      end
    end)
    |> case do
      {:ok, results} -> {:ok, Enum.reverse(results)}
      error -> error
    end
  end

  defp execute_single_tool(session_id, tool_call, tools, state) do
    tool_name = ReqLLM.ToolCall.name(tool_call)
    tool = Enum.find(tools, &(&1.name == tool_name))

    if is_nil(tool) do
      {:error, {:tool_not_found, tool_name}}
    else
      execute_tool_safely(session_id, tool, tool_call, state)
    end
  end

  defp execute_tool_safely(session_id, tool, tool_call, state) do
    input = ReqLLM.ToolCall.args_map(tool_call) || %{}

    # send_tool_notification(session_id, tool.name, input, state)

    # For client request tools, we need to pass the request_callback
    # Store it in the process dictionary so tools can access it
    if state.request_callback do
      Process.put(:request_callback, state.request_callback)
    end

    case ReqLLM.Tool.execute(tool, input) do
      {:ok, result} ->
        # send_tool_result_notification(session_id, tool.name, result, state)

        # Encode result as JSON string
        result_json = Jason.encode!(result)

        tool_result_message =
          ReqLLM.Context.tool_result(
            tool_call.id,
            tool.name,
            result_json
          )

        {:ok, tool_result_message}

      {:error, _reason} = error ->
        error
    end
  rescue
    e ->
      Logger.error(
        "Tool execution exception: #{inspect(e)}\n#{Exception.format_stacktrace(__STACKTRACE__)}"
      )

      {:error, {:tool_execution_exception, e, __STACKTRACE__}}
  end

  defp send_tool_notification(session_id, tool_name, args, state) do
    Logger.info("Executing tool: #{tool_name} with args: #{inspect(args)}")

    # Format tool call info as text with proper spacing
    args_str = Jason.encode!(args, pretty: true)
    tool_text = "\n🔧 Using tool: #{tool_name}\nArguments:\n#{args_str}\n"

    update = %{
      type: "agent_message_chunk",
      content: %{
        type: "text",
        text: tool_text
      }
    }

    params = %{
      session_id: session_id,
      update: update
    }

    if state.notification_callback do
      state.notification_callback.("session/update", params)
    else
      send(TuiAcp.Server, {:send_notification, "session/update", params})
    end
  end

  defp send_tool_result_notification(session_id, tool_name, result, state) do
    Logger.info("Tool #{tool_name} completed with result: #{inspect(result)}")

    # Format tool result as text with proper spacing
    result_str = Jason.encode!(result, pretty: true)
    tool_text = "\n✅ Tool result: #{tool_name}\n#{result_str}\n\n"

    update = %{
      type: "agent_message_chunk",
      content: %{
        type: "text",
        text: tool_text
      }
    }

    params = %{
      session_id: session_id,
      update: update
    }

    if state.notification_callback do
      state.notification_callback.("session/update", params)
    else
      send(TuiAcp.Server, {:send_notification, "session/update", params})
    end
  end

  def execute_weather_tool(args) do
    location = args["location"] || args[:location]
    unit = args["unit"] || args[:unit] || "fahrenheit"

    result = %{
      location: location,
      temperature: Enum.random(50..85),
      unit: unit,
      conditions: Enum.random(["sunny", "partly cloudy", "cloudy", "rainy"]),
      humidity: Enum.random(30..80),
      wind_speed: Enum.random(0..20),
      timestamp: DateTime.utc_now() |> DateTime.to_iso8601()
    }

    {:ok, result}
  end

  def execute_forecast_tool(args) do
    location = args["location"] || args[:location]
    days = args["days"] || args[:days]
    unit = args["unit"] || args[:unit] || "fahrenheit"

    forecast =
      for day <- 1..days do
        %{
          day: day,
          date: Date.utc_today() |> Date.add(day) |> Date.to_iso8601(),
          high: Enum.random(60..90),
          low: Enum.random(40..65),
          conditions: Enum.random(["sunny", "partly cloudy", "cloudy", "rainy", "stormy"]),
          precipitation_chance: Enum.random(0..100)
        }
      end

    result = %{
      location: location,
      unit: unit,
      forecast: forecast
    }

    {:ok, result}
  end

  def execute_read_file_tool(args) do
    path = args["path"] || args[:path]

    Logger.info("Reading file: #{path}")

    # Get the request callback from process dictionary
    request_callback = Process.get(:request_callback)

    if request_callback do
      case TuiAcp.ClientRequest.read_file(request_callback, path) do
        {:ok, content} ->
          {:ok, %{
            path: path,
            content: content,
            size: byte_size(content)
          }}

        {:error, reason} ->
          {:error, "Failed to read file: #{inspect(reason)}"}
      end
    else
      {:error, "Client request capability not available"}
    end
  end

  def execute_write_file_tool(args) do
    path = args["path"] || args[:path]
    content = args["content"] || args[:content]

    Logger.info("Writing file: #{path}")

    # Get the request callback from process dictionary
    request_callback = Process.get(:request_callback)

    if request_callback do
      case TuiAcp.ClientRequest.write_file(request_callback, path, content) do
        :ok ->
          {:ok, %{
            path: path,
            bytes_written: byte_size(content),
            status: "success"
          }}

        {:error, reason} ->
          {:error, "Failed to write file: #{inspect(reason)}"}
      end
    else
      {:error, "Client request capability not available"}
    end
  end

  def execute_grep_search_tool(args) do
    pattern = args["pattern"] || args[:pattern]
    path = args["path"] || args[:path]
    case_sensitive = args["case_sensitive"] || args[:case_sensitive]
    file_pattern = args["file_pattern"] || args[:file_pattern]

    Logger.info("Searching for pattern: #{pattern} in path: #{path || "."}")

    # Get the request callback from process dictionary
    request_callback = Process.get(:request_callback)

    if request_callback do
      opts = [pattern: pattern]
      opts = if path, do: Keyword.put(opts, :path, path), else: opts
      opts = if case_sensitive, do: Keyword.put(opts, :case_sensitive, case_sensitive), else: opts
      opts = if file_pattern, do: Keyword.put(opts, :file_pattern, file_pattern), else: opts

      case TuiAcp.ClientRequest.grep_search(request_callback, opts) do
        {:ok, matches} ->
          {:ok, %{
            pattern: pattern,
            path: path || ".",
            matches: matches,
            count: length(matches)
          }}

        {:error, reason} ->
          {:error, "Failed to perform grep search: #{inspect(reason)}"}
      end
    else
      {:error, "Client request capability not available"}
    end
  end
end
