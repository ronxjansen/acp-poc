defmodule TuiAcp.Utils.Args do
  @moduledoc """
  Utility module for consistent argument extraction from maps.

  Handles the common case where args may come with either atom or string keys,
  providing a unified interface to access values.
  """

  @doc """
  Gets a value from args map, checking both atom and string keys.

  ## Examples

      iex> Args.get(%{"location" => "NYC"}, :location)
      "NYC"

      iex> Args.get(%{location: "NYC"}, :location)
      "NYC"

      iex> Args.get(%{}, :location)
      nil
  """
  @spec get(map(), atom()) :: any()
  def get(args, key) when is_atom(key) do
    args[key] || args[Atom.to_string(key)]
  end

  def get(args, key) when is_binary(key) do
    args[key] || args[String.to_existing_atom(key)]
  rescue
    ArgumentError -> args[key]
  end

  @doc """
  Gets a value from args map with a default fallback.

  ## Examples

      iex> Args.get(%{}, :unit, "fahrenheit")
      "fahrenheit"

      iex> Args.get(%{unit: "celsius"}, :unit, "fahrenheit")
      "celsius"
  """
  @spec get(map(), atom(), any()) :: any()
  def get(args, key, default) do
    case get(args, key) do
      nil -> default
      value -> value
    end
  end

  @doc """
  Extracts multiple keys from args, returning a keyword list.

  Can be called with a list of atom keys, or a keyword list with defaults.

  ## Examples

      iex> Args.take(%{"location" => "NYC", "unit" => "celsius"}, [:location, :unit])
      [location: "NYC", unit: "celsius"]

      iex> Args.take(%{"location" => "NYC"}, location: nil, unit: "fahrenheit")
      [location: "NYC", unit: "fahrenheit"]
  """
  @spec take(map(), [atom()] | keyword()) :: keyword()
  def take(args, keys) when is_list(keys) do
    Enum.map(keys, fn
      {key, default} -> {key, get(args, key, default)}
      key when is_atom(key) -> {key, get(args, key)}
    end)
  end
end
