defmodule TuiAcp.StdioServer do
  @moduledoc """
  A standalone ACP server that reads from stdin and writes to stdout.
  This bypasses ACPex's Port-based transport and uses stdio directly.
  """
  use GenServer
  require Logger

  alias TuiAcp.Agent
  alias TuiAcp.Protocol.JsonRpc

  defstruct [:agent_state, :sessions]

  def start_link(opts \\ []) do
    GenServer.start_link(__MODULE__, opts, name: __MODULE__)
  end

  @impl true
  def init(_opts) do
    {:ok, agent_state} = Agent.init([])

    Logger.debug("Server initialized, starting stdin reader...")

    spawn_link(fn -> read_stdin() end)

    {:ok, %__MODULE__{agent_state: agent_state, sessions: %{}}}
  end

  @impl true
  def handle_info({:stdin, line}, state) do
    case Jason.decode(line) do
      {:ok, message} ->
        handle_message(message, state)

      {:error, reason} ->
        Logger.error("Failed to decode JSON: #{inspect(reason)}")
        {:noreply, state}
    end
  end

  def handle_info({:prompt_complete, msg_id, response, new_agent_state}, state) do
    send_response(msg_id, %{stop_reason: response.stop_reason})
    {:noreply, %{state | agent_state: new_agent_state}}
  end

  def handle_info({:prompt_error, msg_id, reason}, state) do
    send_error(msg_id, -32603, "Prompt processing failed: #{inspect(reason)}")
    {:noreply, state}
  end

  def handle_info({:send_notification, method, params}, state) do
    send_notification(method, params)
    {:noreply, state}
  end

  defp read_stdin do
    case :io.get_line(~c"") do
      :eof ->
        Logger.debug("EOF received, shutting down")
        System.halt(0)

      {:error, reason} ->
        Logger.error("Error reading stdin: #{inspect(reason)}")
        System.halt(1)

      line when is_list(line) or is_binary(line) ->
        line_str = line |> to_string() |> String.trim()

        unless line_str == "" do
          send(__MODULE__, {:stdin, line_str})
        end

        read_stdin()
    end
  end

  defp handle_message(%{"method" => "connection/initialize"} = msg, state) do
    Logger.info("Handling initialize request")

    request = %ACPex.Schema.Connection.InitializeRequest{
      protocol_version: msg["params"]["protocol_version"]
    }

    {:ok, response, new_agent_state} = Agent.handle_initialize(request, state.agent_state)

    send_response(msg["id"], %{
      protocol_version: response.protocol_version,
      agent_capabilities: response.agent_capabilities
    })

    {:noreply, %{state | agent_state: new_agent_state}}
  end

  defp handle_message(%{"method" => "session/new"} = msg, state) do
    Logger.info("Handling new session request")

    request = %ACPex.Schema.Session.NewRequest{}

    {:ok, response, new_agent_state} = Agent.handle_new_session(request, state.agent_state)

    send_response(msg["id"], %{session_id: response.session_id})
    {:noreply, %{state | agent_state: new_agent_state}}
  end

  defp handle_message(%{"method" => "session/prompt"} = msg, state) do
    Logger.info("Handling prompt request")

    params = msg["params"]

    request = %ACPex.Schema.Session.PromptRequest{
      session_id: params["session_id"],
      prompt: params["prompt"]
    }

    server = self()
    msg_id = msg["id"]

    # Use supervised task for proper error handling and crash reports
    Task.Supervisor.start_child(TuiAcp.TaskSupervisor, fn ->
      try do
        {:ok, response, new_agent_state} = Agent.handle_prompt(request, state.agent_state)
        send(server, {:prompt_complete, msg_id, response, new_agent_state})
      rescue
        e ->
          Logger.error("Prompt processing error: #{inspect(e)}\n#{Exception.format_stacktrace(__STACKTRACE__)}")
          send(server, {:prompt_error, msg_id, e})
      end
    end)

    {:noreply, state}
  end

  defp handle_message(%{"method" => method}, state) do
    Logger.warning("Unknown method: #{method}")
    {:noreply, state}
  end

  defp handle_message(msg, state) do
    Logger.warning("Invalid message format: #{inspect(msg)}")
    {:noreply, state}
  end

  defp send_response(id, result) do
    json = JsonRpc.encode_response(id, result)
    IO.puts(json)
  end

  defp send_error(id, code, message) do
    json = JsonRpc.encode_error(id, code, message)
    IO.puts(json)
  end

  def send_notification(method, params) do
    json = JsonRpc.encode_notification(method, params)
    IO.puts(json)
  end
end
