defmodule TuiAcp.Agent.Tools do
  @moduledoc """
  Aggregates all tool definitions for the ACP agent.

  This module serves as a central registry for all available tools,
  making it easy to add new tools by simply adding them to the `all/0` function.
  """

  alias TuiAcp.Agent.Tools.FileSystem
  alias TuiAcp.Agent.Tools.Weather

  @doc """
  Returns all available tool definitions.
  """
  @spec all() :: [any()]
  def all do
    Weather.definitions() ++ FileSystem.definitions()
  end

  @doc """
  Returns only weather-related tools.
  """
  @spec weather() :: [any()]
  def weather, do: Weather.definitions()

  @doc """
  Returns only file system tools.
  """
  @spec file_system() :: [any()]
  def file_system, do: FileSystem.definitions()

  @doc """
  Finds a tool by name.

  Returns `nil` if not found.
  """
  @spec find(String.t()) :: any() | nil
  def find(name) do
    Enum.find(all(), &(&1.name == name))
  end
end
