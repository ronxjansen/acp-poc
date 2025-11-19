defmodule TuiAcp.ClientRequest do
  @moduledoc """
  Helper module for making requests to the client from agent tools.
  """
  require Logger

  @doc """
  Sends a request to the client and waits for the response.

  This is a simplified implementation that uses the request_callback
  passed to the agent during initialization.
  """
  def call(request_callback, method, params, timeout \\ 5000)
      when is_function(request_callback) do
    request_callback.(method, params, timeout)
  end

  @doc """
  Reads a text file from the client's filesystem.
  """
  def read_file(request_callback, path) do
    params = %{
      "path" => path
    }

    case call(request_callback, "fs/read_text_file", params) do
      {:ok, %{"content" => content}} ->
        {:ok, content}

      {:ok, result} ->
        {:error, "Unexpected response format: #{inspect(result)}"}

      {:error, reason} ->
        {:error, reason}
    end
  end

  @doc """
  Writes content to a text file on the client's filesystem.
  """
  def write_file(request_callback, path, content) do
    params = %{
      "path" => path,
      "content" => content
    }

    case call(request_callback, "fs/write_text_file", params) do
      {:ok, _result} ->
        :ok

      {:error, reason} ->
        {:error, reason}
    end
  end

  @doc """
  Performs a grep search in the client's filesystem.

  Options (accepts either map with string keys or keyword list):
  - pattern: Search pattern (required)
  - path: Directory or file to search in (optional, defaults to current directory)
  - case_sensitive: Whether search is case-sensitive (optional, defaults to false)
  - file_pattern: Glob pattern to filter files (optional)
  """
  def grep_search(request_callback, opts) when is_map(opts) do
    pattern = Map.get(opts, "pattern") || Map.get(opts, :pattern)
    path = Map.get(opts, "path") || Map.get(opts, :path) || "."
    case_sensitive = Map.get(opts, "case_sensitive") || Map.get(opts, :case_sensitive) || false
    file_pattern = Map.get(opts, "file_pattern") || Map.get(opts, :file_pattern)

    params = %{
      "pattern" => pattern,
      "path" => path,
      "caseSensitive" => case_sensitive
    }

    params = if file_pattern, do: Map.put(params, "filePattern", file_pattern), else: params

    case call(request_callback, "_fs/grep_search", params) do
      {:ok, %{"matches" => matches}} ->
        {:ok, matches}

      {:ok, result} ->
        {:ok, result}

      {:error, reason} ->
        {:error, reason}
    end
  end

  def grep_search(request_callback, opts) when is_list(opts),
    do: grep_search(request_callback, Map.new(opts))
end
