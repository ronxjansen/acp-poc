defmodule TuiAcp.TcpServer do
  @moduledoc """
  TCP server for ACP protocol communication.
  Listens on a TCP port and spawns ConnectionHandler processes for each client.
  """
  use GenServer
  require Logger

  alias TuiAcp.ConnectionHandler

  @default_port 9090

  def start_link(opts \\ []) do
    GenServer.start_link(__MODULE__, opts, name: __MODULE__)
  end

  @impl true
  def init(opts) do
    port = Keyword.get(opts, :port, @default_port)

    case :gen_tcp.listen(port, [
           :binary,
           packet: :raw,
           active: false,
           reuseaddr: true
         ]) do
      {:ok, listen_socket} ->
        Logger.info("ACP TCP Server listening on port #{port}")
        Logger.info("Waiting for client connections...")

        # Spawn acceptor loop
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

        # Start a ConnectionHandler for this client
        {:ok, pid} = ConnectionHandler.start_link(client_socket)

        # Transfer socket control to the ConnectionHandler
        :ok = :gen_tcp.controlling_process(client_socket, pid)

        # Notify the handler it can now activate the socket
        send(pid, :activate_socket)

        accept_loop(listen_socket)

      {:error, reason} ->
        Logger.error("Failed to accept connection: #{inspect(reason)}")
        accept_loop(listen_socket)
    end
  end
end
