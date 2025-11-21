defmodule TuiAcp.Agent.Tools.Weather do
  @moduledoc """
  Weather-related tool implementations.

  Provides mock weather data for demonstration purposes.
  """

  alias TuiAcp.Utils.Args

  @doc """
  Returns the tool definitions for weather tools.
  """
  @spec definitions() :: [any()]
  def definitions do
    [
      ReqLLM.Tool.new!(
        name: "get_current_weather",
        description: "Get the current weather for a specific location",
        parameter_schema: [
          location: [
            type: :string,
            required: true,
            doc: "The city and state, e.g. San Francisco, CA"
          ],
          unit: [
            type: :string,
            doc: "The temperature unit to use (celsius or fahrenheit)"
          ]
        ],
        callback: {__MODULE__, :execute_current_weather, []}
      ),
      ReqLLM.Tool.new!(
        name: "get_forecast",
        description: "Get the weather forecast for a specific location",
        parameter_schema: [
          location: [
            type: :string,
            required: true,
            doc: "The city and state, e.g. San Francisco, CA"
          ],
          days: [
            type: :integer,
            required: true,
            doc: "Number of days to forecast (1-7)"
          ],
          unit: [
            type: :string,
            doc: "The temperature unit to use (celsius or fahrenheit)"
          ]
        ],
        callback: {__MODULE__, :execute_forecast, []}
      )
    ]
  end

  @doc """
  Executes the get_current_weather tool.

  Returns mock weather data for the given location.
  """
  @spec execute_current_weather(map()) :: {:ok, map()}
  def execute_current_weather(args) do
    location = Args.get(args, :location)
    unit = Args.get(args, :unit, "fahrenheit")

    result = %{
      location: location,
      temperature: Enum.random(50..85),
      unit: unit,
      conditions: Enum.random(["sunny", "partly cloudy", "cloudy", "rainy"]),
      humidity: Enum.random(30..80),
      wind_speed: Enum.random(0..20),
      timestamp: DateTime.utc_now() |> DateTime.to_iso8601()
    }

    {:ok, result}
  end

  @doc """
  Executes the get_forecast tool.

  Returns mock forecast data for the given location and number of days.
  """
  @spec execute_forecast(map()) :: {:ok, map()}
  def execute_forecast(args) do
    location = Args.get(args, :location)
    days = Args.get(args, :days)
    unit = Args.get(args, :unit, "fahrenheit")

    forecast =
      for day <- 1..days do
        %{
          day: day,
          date: Date.utc_today() |> Date.add(day) |> Date.to_iso8601(),
          high: Enum.random(60..90),
          low: Enum.random(40..65),
          conditions: Enum.random(["sunny", "partly cloudy", "cloudy", "rainy", "stormy"]),
          precipitation_chance: Enum.random(0..100)
        }
      end

    result = %{
      location: location,
      unit: unit,
      forecast: forecast
    }

    {:ok, result}
  end
end
