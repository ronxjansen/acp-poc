defmodule TuiAcp.Agent.Context do
  @moduledoc """
  Manages execution context for agent tool calls.

  This module provides a controlled interface for passing context (like request_callback)
  to tool implementations. While it uses process dictionary internally, this is isolated
  here to contain the pattern and make it easier to refactor later.

  ## Why Process Dictionary?

  The ReqLLM.Tool callback mechanism expects a fixed callback signature that doesn't
  easily support passing runtime context. Until we can upstream a change or switch
  libraries, we use process dictionary as a pragmatic solution.

  ## Usage

      # In tool execution (before calling tools)
      Context.with_request_callback(state.request_callback, fn ->
        ReqLLM.Tool.execute(tool, input)
      end)

      # In tool implementation
      case Context.get_request_callback() do
        nil -> {:error, "No client request capability"}
        callback -> callback.("method", params, timeout)
      end
  """

  @request_callback_key :tui_acp_request_callback

  @doc """
  Executes a function with the request_callback available in context.

  Automatically cleans up after execution, even if an exception occurs.
  """
  @spec with_request_callback(function() | nil, (-> result)) :: result when result: any()
  def with_request_callback(callback, fun) do
    previous = Process.get(@request_callback_key)

    try do
      if callback, do: Process.put(@request_callback_key, callback)
      fun.()
    after
      # Restore previous value (or delete if there was none)
      if previous do
        Process.put(@request_callback_key, previous)
      else
        Process.delete(@request_callback_key)
      end
    end
  end

  @doc """
  Gets the current request callback from context.

  Returns `nil` if no callback is available.
  """
  @spec get_request_callback() :: function() | nil
  def get_request_callback do
    Process.get(@request_callback_key)
  end

  @doc """
  Checks if a request callback is available.
  """
  @spec has_request_callback?() :: boolean()
  def has_request_callback? do
    not is_nil(get_request_callback())
  end

  @doc """
  Executes a client request using the current context's callback.

  Returns `{:error, :no_callback}` if no callback is available.
  """
  @spec client_request(String.t(), map(), timeout()) ::
          {:ok, any()} | {:error, any()}
  def client_request(method, params, timeout \\ 30_000) do
    case get_request_callback() do
      nil -> {:error, :no_callback}
      callback -> callback.(method, params, timeout)
    end
  end
end
