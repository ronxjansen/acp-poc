defmodule TuiAcp.Agent.SessionManager do
  @moduledoc """
  Handles session creation and management for the ACP agent.
  """

  @system_message """
  You are a helpful AI assistant with access to various tools. You can:
  - Get weather information using get_current_weather and get_forecast tools
  - Read files from the user's filesystem using the read_file tool
  - Write files to the user's filesystem using the write_file tool
  - Search for text patterns in files using the grep_search tool

  When users ask about weather, use the weather tools. When they ask about files or need to work with the filesystem, use the file tools. When they need to search for text or patterns across files, use the grep_search tool. Always be specific and provide detailed, friendly responses.
  """

  @doc """
  Creates a new session with a unique ID and initial context.

  Returns `{session_id, session_state}`.
  """
  @spec create_session() :: {String.t(), map()}
  def create_session do
    session_id = generate_session_id()

    session_state = %{
      id: session_id,
      context: ReqLLM.Context.new([ReqLLM.Context.system(@system_message)]),
      created_at: DateTime.utc_now()
    }

    {session_id, session_state}
  end

  @doc """
  Generates a unique session ID.
  """
  @spec generate_session_id() :: String.t()
  def generate_session_id do
    "session_" <> (:crypto.strong_rand_bytes(16) |> Base.encode16(case: :lower))
  end

  @doc """
  Gets a session from state by ID.

  Returns `nil` if session not found.
  """
  @spec get_session(map(), String.t()) :: map() | nil
  def get_session(state, session_id) do
    get_in(state, [:sessions, session_id])
  end

  @doc """
  Stores a session in state.
  """
  @spec put_session(map(), String.t(), map()) :: map()
  def put_session(state, session_id, session_state) do
    put_in(state, [:sessions, session_id], session_state)
  end

  @doc """
  Updates the context for a session.
  """
  @spec update_context(map(), String.t(), any()) :: map()
  def update_context(state, session_id, context) do
    put_in(state, [:sessions, session_id, :context], context)
  end

  @doc """
  Returns the system message used for new sessions.
  """
  @spec system_message() :: String.t()
  def system_message, do: @system_message
end
