defmodule TuiAcp.ConnectionHandler do
  @moduledoc """
  GenServer that handles a single client connection.

  Uses active socket mode to receive TCP messages and spawns workers
  for handle_prompt to avoid blocking the GenServer event loop.
  """
  use GenServer
  require Logger

  alias TuiAcp.Agent

  defmodule State do
    @moduledoc false
    defstruct [
      :socket,
      :agent_state,
      :pending_requests,
      buffer: ""
    ]
  end

  @doc """
  Starts a ConnectionHandler for the given socket.
  """
  def start_link(socket) do
    GenServer.start_link(__MODULE__, socket)
  end

  @impl true
  def init(socket) do
    # Don't set active mode yet - wait for :activate_socket message
    # after socket ownership is transferred

    # Get the connection handler PID for use in callbacks
    handler_pid = self()

    # Setup callbacks for the agent
    notification_callback = fn method, params ->
      send_notification(socket, method, params)
    end

    # Callback for client requests - uses GenServer.call to this process
    request_callback = fn method, params, timeout ->
      GenServer.call(handler_pid, {:client_request, method, params}, timeout)
    end

    {:ok, agent_state} =
      Agent.init(
        notification_callback: notification_callback,
        request_callback: request_callback
      )

    state = %State{
      socket: socket,
      agent_state: agent_state,
      pending_requests: %{}
    }

    Logger.info("ConnectionHandler started for socket #{inspect(socket)}")

    {:ok, state}
  end

  # Handle client requests from tools (agent wants to call client)
  @impl true
  def handle_call({:client_request, method, params}, from, state) do
    request_id = :erlang.unique_integer([:positive])

    Logger.debug("Sending client request: #{method} (id: #{request_id})")

    message = %{
      "jsonrpc" => "2.0",
      "id" => request_id,
      "method" => method,
      "params" => params
    }

    # Send the request to client
    send_json(state.socket, message)

    # Store who's waiting for this response
    pending = Map.put(state.pending_requests, request_id, from)

    # Don't reply yet - we'll reply when we get the response from client
    {:noreply, %{state | pending_requests: pending}}
  end

  # Handle socket activation after ownership transfer
  @impl true
  def handle_info(:activate_socket, state) do
    Logger.debug("Activating socket")
    :inet.setopts(state.socket, active: :once, packet: :raw)
    {:noreply, state}
  end

  # Handle incoming TCP data with manual line buffering
  def handle_info({:tcp, socket, data}, state) do
    # Re-enable active mode for next message
    :inet.setopts(socket, active: :once, packet: :raw)

    # Add data to buffer
    new_buffer = state.buffer <> data

    # Process complete lines (ending with \n)
    {lines, remaining_buffer} = extract_lines(new_buffer)

    # Process each complete line
    new_state =
      Enum.reduce(lines, state, fn line, acc_state ->
        Logger.debug("Processing complete line: #{String.slice(line, 0, 100)}...")

        case Jason.decode(line) do
          {:ok, message} ->
            process_message(message, acc_state)

          {:error, reason} ->
            Logger.error("Failed to decode JSON: #{inspect(reason)}")
            Logger.error("JSON line (first 500 chars): #{String.slice(line, 0, 500)}")
            acc_state
        end
      end)

    {:noreply, %{new_state | buffer: remaining_buffer}}
  end

  def handle_info({:tcp_closed, _socket}, state) do
    Logger.info("Client disconnected")
    {:stop, :normal, state}
  end

  def handle_info({:tcp_error, _socket, reason}, state) do
    Logger.error("TCP error: #{inspect(reason)}")
    {:stop, :normal, state}
  end

  # Handle completion of prompt processing from worker
  def handle_info({:prompt_complete, request_id, response, new_agent_state}, state) do
    Logger.debug("Prompt completed for request #{request_id}")

    send_response(state.socket, request_id, %{stopReason: response.stop_reason})

    {:noreply, %{state | agent_state: new_agent_state}}
  end

  # Process different message types
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

  defp process_message(%{"method" => method} = msg, state)
       when method in ["sessions/new", "session/new"] do
    Logger.info("Handling new session request")

    request = %ACPex.Schema.Session.NewRequest{}
    {:ok, response, new_agent_state} = Agent.handle_new_session(request, state.agent_state)

    send_response(state.socket, msg["id"], %{sessionId: response.session_id})

    %{state | agent_state: new_agent_state}
  end

  defp process_message(%{"method" => method} = msg, state)
       when method in ["sessions/prompt", "session/prompt"] do
    Logger.info("Handling prompt request")

    handler_pid = self()
    request_id = msg["id"]
    params = msg["params"]
    agent_state = state.agent_state

    spawn(fn ->
      request = %ACPex.Schema.Session.PromptRequest{
        session_id: params["sessionId"] || params["session_id"],
        prompt: params["prompt"]
      }

      {:ok, response, new_agent_state} = Agent.handle_prompt(request, agent_state)

      send(handler_pid, {:prompt_complete, request_id, response, new_agent_state})
    end)

    state
  end

  # Handle response from client (to our request)
  defp process_message(%{"id" => id, "result" => result}, state)
       when not is_nil(id) and is_map(result) do
    Logger.debug("Received response for request #{id}")

    case Map.pop(state.pending_requests, id) do
      {nil, _} ->
        Logger.warning("Received response for unknown request #{id}")
        state

      {from, pending} ->
        GenServer.reply(from, {:ok, result})
        %{state | pending_requests: pending}
    end
  end

  # Handle error from client
  defp process_message(%{"id" => id, "error" => error}, state) when not is_nil(id) do
    Logger.debug("Received error for request #{id}: #{inspect(error)}")

    case Map.pop(state.pending_requests, id) do
      {nil, _} ->
        Logger.warning("Received error for unknown request #{id}")
        state

      {from, pending} ->
        GenServer.reply(from, {:error, error})
        %{state | pending_requests: pending}
    end
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

  # Helper functions

  # Extract complete lines from buffer (lines ending with \n)
  defp extract_lines(buffer) do
    lines = String.split(buffer, "\n")

    case List.pop_at(lines, -1) do
      {nil, _} ->
        # No lines
        {[], buffer}

      {remaining, complete_lines} when remaining == "" ->
        # All lines were complete (buffer ended with \n)
        {complete_lines, ""}

      {remaining, complete_lines} ->
        # Last part doesn't end with \n, keep it in buffer
        # Trim empty strings from complete lines
        complete = Enum.reject(complete_lines, &(&1 == ""))
        {complete, remaining}
    end
  end

  defp send_response(socket, id, result) do
    message = %{
      "jsonrpc" => "2.0",
      "id" => id,
      "result" => result
    }

    send_json(socket, message)
  end

  defp send_notification(socket, method, params) do
    camel_params = %{
      "sessionId" => params[:session_id] || params["session_id"],
      "update" => params[:update] || params["update"]
    }

    message = %{
      "jsonrpc" => "2.0",
      "method" => method,
      "params" => camel_params
    }

    send_json(socket, message)
  end

  defp send_json(socket, message) do
    json = Jason.encode!(message)
    :gen_tcp.send(socket, json <> "\n")
  end
end
