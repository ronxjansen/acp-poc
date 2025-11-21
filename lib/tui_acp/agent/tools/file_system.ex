defmodule TuiAcp.Agent.Tools.FileSystem do
  @moduledoc """
  File system tool implementations.

  Provides tools for reading, writing, and searching files via client requests.
  """

  require Logger

  alias TuiAcp.Agent.Context
  alias TuiAcp.Utils.Args

  @doc """
  Returns the tool definitions for file system tools.
  """
  @spec definitions() :: [any()]
  def definitions do
    [
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
        callback: {__MODULE__, :execute_read_file, []}
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
        callback: {__MODULE__, :execute_write_file, []}
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
        callback: {__MODULE__, :execute_grep_search, []}
      ),
      ReqLLM.Tool.new!(
        name: "list_dirs",
        description: "List files and directories at a specified path. Returns information about each entry including name, path, size, and whether it's a directory.",
        parameter_schema: [
          path: [
            type: :string,
            required: true,
            doc: "The directory path to list (absolute or relative)"
          ],
          recursive: [
            type: :boolean,
            doc: "Whether to list recursively (defaults to false)"
          ]
        ],
        callback: {__MODULE__, :execute_list_dirs, []}
      )
    ]
  end

  @doc """
  Executes the read_file tool.

  Reads file contents via client request.
  """
  @spec execute_read_file(map()) :: {:ok, map()} | {:error, String.t()}
  def execute_read_file(args) do
    path = Args.get(args, :path)
    Logger.info("Reading file: #{path}")

    with_client_request(fn request_callback ->
      case TuiAcp.ClientRequest.read_file(request_callback, path) do
        {:ok, content} ->
          {:ok, %{path: path, content: content, size: byte_size(content)}}

        {:error, reason} ->
          {:error, "Failed to read file: #{inspect(reason)}"}
      end
    end)
  end

  @doc """
  Executes the write_file tool.

  Writes content to a file via client request.
  """
  @spec execute_write_file(map()) :: {:ok, map()} | {:error, String.t()}
  def execute_write_file(args) do
    path = Args.get(args, :path)
    content = Args.get(args, :content)
    Logger.info("Writing file: #{path}")

    with_client_request(fn request_callback ->
      case TuiAcp.ClientRequest.write_file(request_callback, path, content) do
        :ok ->
          {:ok, %{path: path, bytes_written: byte_size(content), status: "success"}}

        {:error, reason} ->
          {:error, "Failed to write file: #{inspect(reason)}"}
      end
    end)
  end

  @doc """
  Executes the grep_search tool.

  Searches for patterns in files via client request.
  """
  @spec execute_grep_search(map()) :: {:ok, map()} | {:error, String.t()}
  def execute_grep_search(args) do
    pattern = Args.get(args, :pattern)
    path = Args.get(args, :path)
    case_sensitive = Args.get(args, :case_sensitive)
    file_pattern = Args.get(args, :file_pattern)

    Logger.info("Searching for pattern: #{pattern} in path: #{path || "."}")

    with_client_request(fn request_callback ->
      opts =
        [pattern: pattern]
        |> maybe_put(:path, path)
        |> maybe_put(:case_sensitive, case_sensitive)
        |> maybe_put(:file_pattern, file_pattern)

      case TuiAcp.ClientRequest.grep_search(request_callback, opts) do
        {:ok, matches} ->
          {:ok, %{pattern: pattern, path: path || ".", matches: matches, count: length(matches)}}

        {:error, reason} ->
          {:error, "Failed to perform grep search: #{inspect(reason)}"}
      end
    end)
  end

  @doc """
  Executes the list_dirs tool.

  Lists files and directories at the specified path via client request.
  """
  @spec execute_list_dirs(map()) :: {:ok, map()} | {:error, String.t()}
  def execute_list_dirs(args) do
    path = Args.get(args, :path)
    recursive = Args.get(args, :recursive, false)

    Logger.info("Listing directories: #{path}, recursive: #{recursive}")

    with_client_request(fn request_callback ->
      case TuiAcp.ClientRequest.list_dirs(request_callback, path, recursive) do
        {:ok, entries} ->
          {:ok, %{path: path, recursive: recursive, entries: entries, count: length(entries)}}

        {:error, reason} ->
          {:error, "Failed to list directories: #{inspect(reason)}"}
      end
    end)
  end

  # Helper to execute a function with the client request callback
  defp with_client_request(fun) do
    case Context.get_request_callback() do
      nil -> {:error, "Client request capability not available"}
      callback -> fun.(callback)
    end
  end

  # Helper to conditionally add a key to a keyword list
  defp maybe_put(opts, _key, nil), do: opts
  defp maybe_put(opts, key, value), do: Keyword.put(opts, key, value)
end
