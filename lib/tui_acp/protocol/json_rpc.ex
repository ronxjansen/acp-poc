defmodule TuiAcp.Protocol.JsonRpc do
  @moduledoc """
  Shared JSON-RPC 2.0 message encoding utilities.

  Provides consistent message formatting for both TCP and stdio transports.
  """

  @doc """
  Encodes a successful response.

  ## Examples

      iex> JsonRpc.encode_response(1, %{status: "ok"})
      ~s({"jsonrpc":"2.0","id":1,"result":{"status":"ok"}})
  """
  @spec encode_response(any(), any()) :: String.t()
  def encode_response(id, result) do
    %{
      "jsonrpc" => "2.0",
      "id" => id,
      "result" => result
    }
    |> Jason.encode!()
  end

  @doc """
  Encodes an error response.

  ## Examples

      iex> JsonRpc.encode_error(1, -32600, "Invalid Request")
      ~s({"jsonrpc":"2.0","id":1,"error":{"code":-32600,"message":"Invalid Request"}})
  """
  @spec encode_error(any(), integer(), String.t()) :: String.t()
  def encode_error(id, code, message) do
    %{
      "jsonrpc" => "2.0",
      "id" => id,
      "error" => %{
        "code" => code,
        "message" => message
      }
    }
    |> Jason.encode!()
  end

  @doc """
  Encodes a notification (no id field).

  ## Examples

      iex> JsonRpc.encode_notification("session/update", %{session_id: "123"})
      ~s({"jsonrpc":"2.0","method":"session/update","params":{"session_id":"123"}})
  """
  @spec encode_notification(String.t(), map()) :: String.t()
  def encode_notification(method, params) do
    %{
      "jsonrpc" => "2.0",
      "method" => method,
      "params" => params
    }
    |> Jason.encode!()
  end

  @doc """
  Encodes a request message (for client requests from agent).
  """
  @spec encode_request(any(), String.t(), map()) :: String.t()
  def encode_request(id, method, params) do
    %{
      "jsonrpc" => "2.0",
      "id" => id,
      "method" => method,
      "params" => params
    }
    |> Jason.encode!()
  end

  @doc """
  Builds a response map (for when you need the map, not the encoded string).
  """
  @spec response(any(), any()) :: map()
  def response(id, result) do
    %{
      "jsonrpc" => "2.0",
      "id" => id,
      "result" => result
    }
  end

  @doc """
  Builds an error map.
  """
  @spec error(any(), integer(), String.t()) :: map()
  def error(id, code, message) do
    %{
      "jsonrpc" => "2.0",
      "id" => id,
      "error" => %{
        "code" => code,
        "message" => message
      }
    }
  end

  @doc """
  Builds a notification map.
  """
  @spec notification(String.t(), map()) :: map()
  def notification(method, params) do
    %{
      "jsonrpc" => "2.0",
      "method" => method,
      "params" => params
    }
  end

  @doc """
  Converts snake_case keys to camelCase for ACP protocol compliance.

  ## Examples

      iex> JsonRpc.to_camel_case(%{session_id: "123", stop_reason: "done"})
      %{"sessionId" => "123", "stopReason" => "done"}
  """
  @spec to_camel_case(map()) :: map()
  def to_camel_case(map) when is_map(map) do
    Map.new(map, fn {key, value} ->
      camel_key = key |> to_string() |> camelize()
      camel_value = if is_map(value), do: to_camel_case(value), else: value
      {camel_key, camel_value}
    end)
  end

  defp camelize(string) do
    string
    |> String.split("_")
    |> Enum.with_index()
    |> Enum.map(fn
      {word, 0} -> word
      {word, _} -> String.capitalize(word)
    end)
    |> Enum.join()
  end

  @doc """
  Converts camelCase keys to snake_case atoms for internal use.

  ## Examples

      iex> JsonRpc.to_snake_case(%{"sessionId" => "123", "stopReason" => "done"})
      %{session_id: "123", stop_reason: "done"}
  """
  @spec to_snake_case(map()) :: map()
  def to_snake_case(map) when is_map(map) do
    Map.new(map, fn {key, value} ->
      snake_key = key |> to_string() |> snakify() |> String.to_atom()
      snake_value = if is_map(value), do: to_snake_case(value), else: value
      {snake_key, snake_value}
    end)
  end

  defp snakify(string) do
    string
    |> String.replace(~r/([A-Z])/, "_\\1")
    |> String.downcase()
    |> String.trim_leading("_")
  end
end
