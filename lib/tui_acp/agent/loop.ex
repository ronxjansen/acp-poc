defmodule TuiAcp.Agent.Loop do
  @moduledoc """
  Implements the agent loop that handles LLM interactions and tool execution.
  """

  require Logger

  alias TuiAcp.Agent.Executor
  alias TuiAcp.Agent.Tools

  @default_max_turns 10
  # @model "openai:gpt-4o-mini"
  @model "anthropic:claude-haiku-4-5"

  @doc """
  Runs the agent loop, processing user input and executing tools as needed.

  Options:
  - `:max_turns` - Maximum number of tool execution turns (default: 10)

  Returns `{:ok, updated_context, assistant_message}` or `{:error, reason}`.
  """
  @spec run(String.t(), any(), map(), keyword()) ::
          {:ok, any(), String.t()} | {:error, any()}
  def run(session_id, context, state, opts \\ []) do
    max_turns = Keyword.get(opts, :max_turns, @default_max_turns)
    run_loop(session_id, context, state, 0, max_turns)
  end

  defp run_loop(_session_id, context, _state, current_turn, max_turns)
       when current_turn >= max_turns do
    content = extract_text_from_message(List.last(context.messages)) || "Max turns reached."
    {:ok, context, content}
  end

  defp run_loop(session_id, context, state, current_turn, max_turns) do
    tools = Tools.all()

    case ReqLLM.generate_text(@model, context, tools: tools, temperature: 0.7) do
      {:ok, response} ->
        if has_tool_calls?(response.context) do
          handle_tool_calls(session_id, response.context, tools, state, current_turn, max_turns)
        else
          content =
            ReqLLM.Response.text(response) || "I apologize, but I couldn't generate a response."

          {:ok, response.context, content}
        end

      {:error, reason} ->
        Logger.error("Error calling LLM via ReqLLM: #{inspect(reason)}")
        {:error, reason}
    end
  rescue
    e ->
      Logger.error("Error calling LLM: #{inspect(e)}")
      {:error, e}
  end

  defp handle_tool_calls(session_id, context, tools, state, current_turn, max_turns) do
    case Executor.execute_tools_in_context(session_id, context, tools, state) do
      {:ok, updated_context} ->
        run_loop(session_id, updated_context, state, current_turn + 1, max_turns)

      {:error, reason} ->
        {:error, reason}
    end
  end

  @doc """
  Checks if the latest message in the context contains tool calls.
  """
  @spec has_tool_calls?(any()) :: boolean()
  def has_tool_calls?(context) do
    case List.last(context.messages) do
      nil ->
        false

      message ->
        is_list(message.tool_calls) and length(message.tool_calls) > 0
    end
  end

  # Extract text content from a Message struct
  defp extract_text_from_message(nil), do: nil

  defp extract_text_from_message(%{content: content}) when is_list(content) do
    text =
      content
      |> Enum.filter(&(&1.type == :text))
      |> Enum.map_join("", & &1.text)

    if text == "", do: nil, else: text
  end

  defp extract_text_from_message(_), do: nil
end
