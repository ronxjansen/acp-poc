defmodule TuiAcp.Application do
  @moduledoc false

  use Application
  require Logger

  @impl true
  def start(_type, _args) do
    load_env()

    ReqLLM.put_key(:anthropic_api_key, System.get_env("ANTHROPIC_API_KEY"))

    server_mode = System.get_env("ACP_SERVER_MODE", "tcp")
    port = System.get_env("ACP_PORT", "9090") |> String.to_integer()

    children =
      case server_mode do
        "stdio" ->
          Logger.info("Starting Agent ACP Server (stdio mode)...")

          [TuiAcp.StdioServer]

        "tcp" ->
          Logger.info("Starting Agent ACP Server (TCP mode)...")

          [{TuiAcp.TcpServer, port: port}]

        _ ->
          raise "Invalid ACP_SERVER_MODE: #{server_mode}. Use 'tcp' or 'stdio'"
      end

    opts = [strategy: :one_for_one, name: TuiAcp.Supervisor]
    Supervisor.start_link(children, opts)
  end

  defp load_env do
    env_file = Path.join(File.cwd!(), ".env")

    if File.exists?(env_file) do
      case Dotenvy.source(env_file) do
        {:ok, _} ->
          Logger.info("Loaded environment variables from .env file")

        {:error, reason} ->
          Logger.error("Could not load .env file: #{inspect(reason)}")
      end
    end
  end
end
