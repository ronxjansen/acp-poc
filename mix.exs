defmodule TuiAcp.MixProject do
  use Mix.Project

  def project do
    [
      app: :tui_acp,
      version: "0.1.0",
      elixir: "~> 1.18",
      start_permanent: Mix.env() == :prod,
      deps: deps()
    ]
  end

  # Run "mix help compile.app" to learn about applications.
  def application do
    [
      extra_applications: [:logger, :crypto],
      mod: {TuiAcp.Application, []}
    ]
  end

  # Run "mix help deps" to learn about dependencies.
  defp deps do
    [
      {:req_llm, "~> 1.0"},
      {:acpex, "~> 0.1.0"},
      {:req, "~> 0.5"},
      {:jason, "~> 1.4"},
      {:dotenvy, "~> 1.1"}
    ]
  end
end
