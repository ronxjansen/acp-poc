defmodule TuiAcp.Agent do
  @moduledoc """
  ACP Agent facade that delegates to specialized submodules.

  This module implements the `ACPex.Agent` behaviour and coordinates
  between the various agent components:

  - `TuiAcp.Agent.SessionManager` - Session creation and management
  - `TuiAcp.Agent.Loop` - Agent loop for LLM interactions
  - `TuiAcp.Agent.Executor` - Tool execution
  - `TuiAcp.Agent.Tools` - Tool definitions
  - `TuiAcp.Agent.Context` - Execution context management
  """
  @behaviour ACPex.Agent

  require Logger

  alias TuiAcp.Agent.Loop
  alias TuiAcp.Agent.SessionManager

  # ============================================================================
  # ACPex.Agent Behaviour Implementation
  # ============================================================================

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
    {session_id, session_state} = SessionManager.create_session()
    new_state = SessionManager.put_session(state, session_id, session_state)

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

    session = SessionManager.get_session(state, session_id)
    updated_context = ReqLLM.Context.append(session.context, ReqLLM.Context.user(user_message))

    case Loop.run(session_id, updated_context, state, max_turns: 10) do
      {:ok, new_context, assistant_message} ->
        state = SessionManager.update_context(state, session_id, new_context)
        send_text_chunk(session_id, assistant_message, state)

        response = %ACPex.Schema.Session.PromptResponse{stop_reason: "done"}
        {:ok, response, state}

      {:error, reason} ->
        error_msg = "Error processing request: #{inspect(reason)}"
        send_text_chunk(session_id, error_msg, state)

        response = %ACPex.Schema.Session.PromptResponse{stop_reason: "error"}
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

  # ============================================================================
  # Private Helpers
  # ============================================================================

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

  # Unified notification helper
  defp send_session_notification(session_id, update, state) do
    params = %{session_id: session_id, update: update}

    if state.notification_callback do
      state.notification_callback.("session/update", params)
    else
      send(TuiAcp.Server, {:send_notification, "session/update", params})
    end
  end

  defp send_text_chunk(session_id, text, state) do
    update = %{
      type: "agent_message_chunk",
      content: %{type: "text", text: text}
    }

    send_session_notification(session_id, update, state)
  end

  # ============================================================================
  # Public API for Tool Notifications (optional, for verbose/debug mode)
  # ============================================================================

  @doc false
  def send_tool_notification(session_id, tool_name, args, state) do
    Logger.info("Executing tool: #{tool_name} with args: #{inspect(args)}")

    args_str = Jason.encode!(args, pretty: true)
    tool_text = "\n🔧 Using tool: #{tool_name}\nArguments:\n#{args_str}\n"

    update = %{
      type: "agent_message_chunk",
      content: %{type: "text", text: tool_text}
    }

    send_session_notification(session_id, update, state)
  end

  @doc false
  def send_tool_result_notification(session_id, tool_name, result, state) do
    Logger.info("Tool #{tool_name} completed with result: #{inspect(result)}")

    result_str = Jason.encode!(result, pretty: true)
    tool_text = "\n✅ Tool result: #{tool_name}\n#{result_str}\n\n"

    update = %{
      type: "agent_message_chunk",
      content: %{type: "text", text: tool_text}
    }

    send_session_notification(session_id, update, state)
  end
end
