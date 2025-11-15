defmodule TuiAcp.TcpServer do
  @moduledoc """
  TCP server for ACP protocol communication.
  Listens on a TCP port and handles multiple client connections.
  """
  use GenServer
  require Logger

  alias TuiAcp.Agent

  @default_port 9090

  defmodule ConnectionState do
    defstruct [:socket, :agent_state, :sessions, :buffer]
  end

  def start_link(opts \\ []) do
    GenServer.start_link(__MODULE__, opts, name: __MODULE__)
  end

  @impl true
  def init(opts) do
    port = Keyword.get(opts, :port, @default_port)

    case :gen_tcp.listen(port, [
           :binary,
           packet: :line,
           active: false,
           reuseaddr: true
         ]) do
      {:ok, listen_socket} ->
        Logger.info("ACP TCP Server listening on port #{port}")
        Logger.info("Waiting for client connections...")

        spawn_link(fn -> accept_loop(listen_socket) end)

        {:ok, %{listen_socket: listen_socket, port: port}}

      {:error, :eaddrinuse} ->
        Logger.error("ERROR: Port #{port} is already in use!")
        {:stop, {:port_in_use, port}}

      {:error, reason} ->
        Logger.error("ERROR: Failed to start TCP server on port #{port}: #{inspect(reason)}")
        {:stop, {:tcp_error, reason}}
    end
  end

  defp accept_loop(listen_socket) do
    case :gen_tcp.accept(listen_socket) do
      {:ok, client_socket} ->
        Logger.info("Client connected")

        spawn(fn -> handle_client(client_socket) end)

        accept_loop(listen_socket)

      {:error, reason} ->
        Logger.error("Failed to accept connection: #{inspect(reason)}")
        accept_loop(listen_socket)
    end
  end

  defp handle_client(socket) do
    notification_callback = fn method, params ->
      send_notification(socket, method, params)
    end

    {:ok, agent_state} = Agent.init(notification_callback: notification_callback)

    state = %ConnectionState{
      socket: socket,
      agent_state: agent_state,
      sessions: %{},
      buffer: ""
    }

    read_loop(state)
  end

  defp read_loop(state) do
    case :gen_tcp.recv(state.socket, 0) do
      {:ok, data} ->
        line = String.trim(data)

        unless line == "" do
          new_state = handle_message(line, state)
          read_loop(new_state)
        else
          read_loop(state)
        end

      {:error, :closed} ->
        Logger.info("Client disconnected")
        :ok

      {:error, reason} ->
        Logger.error("Error reading from socket: #{inspect(reason)}")
        :ok
    end
  end

  defp handle_message(line, state) do
    case Jason.decode(line) do
      {:ok, message} ->
        process_message(message, state)

      {:error, reason} ->
        Logger.error("Failed to decode JSON: #{inspect(reason)}")
        state
    end
  end

  # Handle both "initialize" and "connection/initialize" for compatibility
  defp process_message(%{"method" => method} = msg, state)
       when method in ["initialize", "connection/initialize"] do
    Logger.info("Handling initialize request")

    request = %ACPex.Schema.Connection.InitializeRequest{
      protocol_version: msg["params"]["protocolVersion"] || msg["params"]["protocol_version"]
    }

    {:ok, response, new_agent_state} = Agent.handle_initialize(request, state.agent_state)

    send_response(state.socket, msg["id"], %{
      protocolVersion: response.protocol_version,
      agentCapabilities: response.agent_capabilities
    })

    %{state | agent_state: new_agent_state}
  end

  # Handle both "sessions/new" and "session/new"
  defp process_message(%{"method" => method} = msg, state)
       when method in ["sessions/new", "session/new"] do
    Logger.info("Handling new session request")

    request = %ACPex.Schema.Session.NewRequest{}

    {:ok, response, new_agent_state} = Agent.handle_new_session(request, state.agent_state)

    send_response(state.socket, msg["id"], %{sessionId: response.session_id})

    %{state | agent_state: new_agent_state}
  end

  # Handle both "sessions/prompt" and "session/prompt"
  defp process_message(%{"method" => method} = msg, state)
       when method in ["sessions/prompt", "session/prompt"] do
    Logger.info("Handling prompt request")

    params = msg["params"]

    request = %ACPex.Schema.Session.PromptRequest{
      session_id: params["sessionId"] || params["session_id"],
      prompt: params["prompt"]
    }

    {:ok, response, new_agent_state} = Agent.handle_prompt(request, state.agent_state)
    send_response(state.socket, msg["id"], %{stopReason: response.stop_reason})

    %{state | agent_state: new_agent_state}
  end

  defp process_message(%{"method" => method} = msg, state) do
    Logger.warning("Unknown method: #{method}")
    Logger.warning("Full message: #{inspect(msg)}")
    state
  end

  defp process_message(msg, state) do
    Logger.warning("Invalid message format: #{inspect(msg)}")
    state
  end

  defp send_response(socket, id, result) do
    message = %{
      "jsonrpc" => "2.0",
      "id" => id,
      "result" => result
    }

    send_message(socket, message)
  end

  def send_notification(socket, method, params) do
    # Convert params keys to camelCase for Go SDK compatibility
    camel_params = %{
      "sessionId" => params[:session_id] || params["session_id"],
      "update" => params[:update] || params["update"]
    }

    message = %{
      "jsonrpc" => "2.0",
      "method" => method,
      "params" => camel_params
    }

    send_message(socket, message)
  end

  defp send_message(socket, message) do
    json = Jason.encode!(message)
    :gen_tcp.send(socket, json <> "\n")
  end
end
