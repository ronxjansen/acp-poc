defmodule TuiAcp.Protocol.Handler do
  @moduledoc """
  Shared protocol message handling logic.

  Provides a unified interface for handling ACP protocol messages,
  used by both TCP and stdio transports.
  """

  require Logger

  alias TuiAcp.Agent
  alias TuiAcp.Protocol.JsonRpc

  @type transport :: :tcp | :stdio
  @type send_fn :: (String.t() -> :ok | {:error, term()})
  @type handler_context :: %{
          send_fn: send_fn(),
          transport: transport(),
          handler_pid: pid()
        }

  @doc """
  Handles an initialize request.

  Returns `{:ok, response_json, new_agent_state}` on success.
  """
  @spec handle_initialize(map(), map(), handler_context()) ::
          {:ok, String.t(), map()}
  def handle_initialize(msg, agent_state, ctx) do
    Logger.info("Handling initialize request")

    params = msg["params"] || %{}

    request = %ACPex.Schema.Connection.InitializeRequest{
      protocol_version: params["protocolVersion"] || params["protocol_version"]
    }

    {:ok, response, new_agent_state} = Agent.handle_initialize(request, agent_state)

    result =
      case ctx.transport do
        :tcp ->
          # TCP uses camelCase
          %{
            protocolVersion: response.protocol_version,
            agentCapabilities: response.agent_capabilities
          }

        :stdio ->
          # stdio uses snake_case
          %{
            protocol_version: response.protocol_version,
            agent_capabilities: response.agent_capabilities
          }
      end

    response_json = JsonRpc.encode_response(msg["id"], result)
    {:ok, response_json, new_agent_state}
  end

  @doc """
  Handles a new session request.

  Returns `{:ok, response_json, new_agent_state}` on success.
  """
  @spec handle_new_session(map(), map(), handler_context()) ::
          {:ok, String.t(), map()}
  def handle_new_session(msg, agent_state, ctx) do
    Logger.info("Handling new session request")

    request = %ACPex.Schema.Session.NewRequest{}
    {:ok, response, new_agent_state} = Agent.handle_new_session(request, agent_state)

    result =
      case ctx.transport do
        :tcp -> %{sessionId: response.session_id}
        :stdio -> %{session_id: response.session_id}
      end

    response_json = JsonRpc.encode_response(msg["id"], result)
    {:ok, response_json, new_agent_state}
  end

  @doc """
  Handles a prompt request asynchronously.

  Spawns a supervised task to process the prompt and sends results
  back to the handler process.

  Returns `:ok` immediately (response sent via message to handler_pid).
  """
  @spec handle_prompt_async(map(), map(), handler_context()) :: :ok
  def handle_prompt_async(msg, agent_state, ctx) do
    Logger.info("Handling prompt request")

    params = msg["params"] || %{}
    msg_id = msg["id"]
    handler_pid = ctx.handler_pid

    request = %ACPex.Schema.Session.PromptRequest{
      session_id: params["sessionId"] || params["session_id"],
      prompt: params["prompt"]
    }

    # Use Task.Supervisor for proper supervision
    Task.Supervisor.start_child(TuiAcp.TaskSupervisor, fn ->
      try do
        {:ok, response, new_agent_state} = Agent.handle_prompt(request, agent_state)
        send(handler_pid, {:prompt_complete, msg_id, response, new_agent_state})
      rescue
        e ->
          Logger.error("Prompt processing error: #{inspect(e)}\n#{Exception.format_stacktrace(__STACKTRACE__)}")
          send(handler_pid, {:prompt_error, msg_id, e})
      end
    end)

    :ok
  end

  @doc """
  Builds a session update notification for text chunks.
  """
  @spec build_text_chunk_notification(String.t(), String.t(), transport()) :: String.t()
  def build_text_chunk_notification(session_id, text, transport) do
    update = %{
      type: "agent_message_chunk",
      content: %{
        type: "text",
        text: text
      }
    }

    params =
      case transport do
        :tcp -> %{"sessionId" => session_id, "update" => update}
        :stdio -> %{session_id: session_id, update: update}
      end

    JsonRpc.encode_notification("session/update", params)
  end

  @doc """
  Parses a method string and returns a normalized method atom.
  """
  @spec normalize_method(String.t()) :: atom()
  def normalize_method(method) do
    case method do
      m when m in ["initialize", "connection/initialize"] -> :initialize
      m when m in ["sessions/new", "session/new"] -> :new_session
      m when m in ["sessions/prompt", "session/prompt"] -> :prompt
      _ -> :unknown
    end
  end

  @doc """
  Extracts complete lines from a buffer (for TCP line-based protocol).

  Returns `{complete_lines, remaining_buffer}`.

  Uses efficient string operations instead of List.pop_at(-1).
  """
  @spec extract_lines(String.t()) :: {[String.t()], String.t()}
  def extract_lines(buffer) do
    case :binary.matches(buffer, "\n") do
      [] ->
        # No newlines, entire buffer is incomplete
        {[], buffer}

      matches ->
        # Find the last newline position
        {last_pos, _len} = List.last(matches)
        complete_part = binary_part(buffer, 0, last_pos)
        remaining = binary_part(buffer, last_pos + 1, byte_size(buffer) - last_pos - 1)

        lines =
          complete_part
          |> String.split("\n")
          |> Enum.reject(&(&1 == ""))

        {lines, remaining}
    end
  end
end
