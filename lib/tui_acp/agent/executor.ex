defmodule TuiAcp.Agent.Executor do
  @moduledoc """
  Handles tool execution for the ACP agent.

  Responsible for safely executing tools within the proper context,
  handling errors, and returning results.
  """

  require Logger

  alias TuiAcp.Agent.Context

  @doc """
  Executes all tool calls from the current context.

  Returns `{:ok, updated_context}` or `{:error, reason}`.
  """
  @spec execute_tools_in_context(String.t(), any(), [any()], map()) ::
          {:ok, any()} | {:error, any()}
  def execute_tools_in_context(session_id, context, tools, state) do
    latest_message = List.last(context.messages)
    tool_calls = latest_message.tool_calls || []

    case execute_all_tools(session_id, tool_calls, tools, state) do
      {:ok, tool_results} ->
        updated_context = ReqLLM.Context.append(context, tool_results)
        {:ok, updated_context}

      {:error, _reason} = error ->
        error
    end
  end

  @doc """
  Executes a list of tool calls sequentially.

  Returns `{:ok, [tool_results]}` or `{:error, reason}` on first failure.
  """
  @spec execute_all_tools(String.t(), [any()], [any()], map()) ::
          {:ok, [any()]} | {:error, any()}
  def execute_all_tools(session_id, tool_calls, tools, state) do
    tool_calls
    |> Enum.reduce_while({:ok, []}, fn tool_call, {:ok, results} ->
      case execute_single_tool(session_id, tool_call, tools, state) do
        {:ok, result} ->
          {:cont, {:ok, [result | results]}}

        {:error, _reason} = error ->
          {:halt, error}
      end
    end)
    |> case do
      {:ok, results} -> {:ok, Enum.reverse(results)}
      error -> error
    end
  end

  @doc """
  Executes a single tool call.

  Looks up the tool by name and delegates to `execute_tool_safely/4`.
  """
  @spec execute_single_tool(String.t(), any(), [any()], map()) ::
          {:ok, any()} | {:error, any()}
  def execute_single_tool(session_id, tool_call, tools, state) do
    tool_name = ReqLLM.ToolCall.name(tool_call)
    tool = Enum.find(tools, &(&1.name == tool_name))

    if is_nil(tool) do
      {:error, {:tool_not_found, tool_name}}
    else
      execute_tool_safely(session_id, tool, tool_call, state)
    end
  end

  @doc """
  Executes a tool with proper error handling and context setup.

  Uses the Context module to make the request_callback available to tools
  that need to make client requests.
  """
  @spec execute_tool_safely(String.t(), any(), any(), map()) ::
          {:ok, any()} | {:error, any()}
  def execute_tool_safely(_session_id, tool, tool_call, state) do
    input = ReqLLM.ToolCall.args_map(tool_call) || %{}

    # Use Context wrapper to make request_callback available to tools
    # This isolates the process dictionary usage and ensures proper cleanup
    Context.with_request_callback(state.request_callback, fn ->
      case ReqLLM.Tool.execute(tool, input) do
        {:ok, result} ->
          result_json = Jason.encode!(result)

          tool_result_message =
            ReqLLM.Context.tool_result(
              tool_call.id,
              tool.name,
              result_json
            )

          {:ok, tool_result_message}

        {:error, _reason} = error ->
          error
      end
    end)
  rescue
    e ->
      Logger.error(
        "Tool execution exception: #{inspect(e)}\n#{Exception.format_stacktrace(__STACKTRACE__)}"
      )

      {:error, {:tool_execution_exception, e, __STACKTRACE__}}
  end
end
