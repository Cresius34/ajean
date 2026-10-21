# AJEAN

**English** · [Français](README.fr.md)

![AJEAN web interface](docs/ui.png)

> **Derived project, personal use.** This repository is a **fork** of
> [**nathaninline/ajean**](https://github.com/nathaninline/ajean), created and maintained by
> **nathaninline** — the upstream project, and the only source of published AJEAN releases.
> Everything below describes AJEAN **in general**. This fork carries a **private patch line**
> (**0.18.4.2 Cresius**) for one specific machine: the **Strata** MoE engine on AMD,
> embedding search, Crawl4AI fixes. Those additions are **not** upstream releases, are
> **not** tested elsewhere, and are **not** guaranteed to work anywhere else. To install
> AJEAN, use the releases published on
> [nathaninline/ajean](https://github.com/nathaninline/ajean/releases/latest).
> The MoE engine used here comes from [Niko1221/Strata](https://github.com/Niko1221/Strata).
> What exactly this repository changes: [NOTICE.md](NOTICE.md).

**Run your own AI at home.** AJEAN is a single program that turns a computer with a graphics card (or a plain CPU) into a complete AI assistant: a chat interface, memory, tools, web access, scheduled tasks, and access from your phone, with nothing leaving your machine.

AJEAN handles everything around the model. The model itself runs on [llama.cpp](https://github.com/ggml-org/llama.cpp), which AJEAN installs and keeps up to date for you.

- **One file, no dependencies.** No Docker, no Python, no runtime to install. Linux, Windows and macOS.
- **An assistant, not just a model.** It remembers, organizes work into projects, runs commands, reads the web, drives a browser, and works on its own on a schedule.
- **Your data stays yours.** Everything runs on your machine. Memory and conversations can be encrypted at rest, and remote access is encrypted end to end.
- **Use it from anywhere.** The same conversations on your computer and your phone, live.
- **Plugs into your tools.** An OpenAI-compatible endpoint for any third-party app.

---
