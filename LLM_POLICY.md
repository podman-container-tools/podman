![PODMAN logo](https://raw.githubusercontent.com/containers/common/main/logos/podman-logo-full-vert.png)
# Podman LLM (AI) Development Policy

This document is based on [Jellyfin LLM Policy](https://jellyfin.org/docs/general/contributing/llm-policies/)
and licensed under [CC-BY-ND-4.0](http://creativecommons.org/licenses/by-nd/4.0/).

LLMs such as Claude and ChatGPT are powerful development tools. They can
help both experienced and new developers. However, they also introduce risks.

Podman has always prioritized code quality (readability, simplicity, and conciseness)
and friendly communication. Our small team maintains these standards manually. As LLM
usage grows within the Podman community, this policy clarifies our expectations for
contributions and communication across all official projects and spaces.

## No LLM-Generated Direct Communication

LLM output must **not** be used verbatim in:

* Issues or comments
* Pull request bodies, comments, or commit messages.
* Forum or chat posts
* Security and vulnerability reports

All communication must be written in your own words. You must understand what you are submitting.

LLM-written content is often long, impersonal, and error-prone. As a small team with limited
resources, we are unable to spend time reviewing unclear submissions or responding to impersonal
comments.

* Exception: If you use an LLM to translate your thoughts into English, clearly state this
  (e.g., “Translated with an LLM from MyLanguage”).
* Exception: The LLM-based bot can be configured by the maintainers to review the PRs
  and suggest changes. These changes are only suggestions and might be wrong. It’s up
  to the contributor and maintainer to decide whether the particular suggestion makes sense.

Repeated violations may result in the closure or deletion of the submission or in a permanent ban from Podman projects.

## LLM Code Contributions

LLMs may assist with code, but you are fully responsible for what you submit.

### Requirements

* Follow all guidelines in [CONTRIBUTING.md](CONTRIBUTING.md).
* Keep changes concise and focused.
* Match existing formatting and quality standards.
* Remove unnecessary comments, poor structure, whitespace issues, and editor/LLM metadata files (e.g., .claude configs).
* The code must build, run, and pass tests before review begins.
* Explicitly test the functionality you modify.

### Understanding and Ownership

You must:
* Review all generated code.
* Clearly explain (in your own words) what the change does and why in both the PR body and commit message.
* Be able to discuss and justify your changes during review.

Submitting “vibe-coded” or poorly understood changes will result in rejection after a few attempts to correct them.

### Handling Review Feedback

Do not paste reviewer feedback into an LLM and resubmit whatever it generates.

Please engage in the review process by:
* Responding thoughtfully to feedback using your own words.
* Making minimal, targeted changes to address comments.
* Understanding the implementation of required changes.

### Final Discretion

Maintainers have final discretion. PRs that are too large, overly complex, poorly structured,
or difficult to review may be rejected after a few attempts to correct them — regardless
of whether LLMs were used.

Violations may result in the closure or deletion of the submission, or in a permanent ban from Podman projects.

## The Golden Rule
Do not prompt an LLM vaguely. Do not commit the LLM results unchanged. And do not submit them as-is.

Using LLMs as a tool is completely fine. Using them as a replacement for understanding,
responsibility, and craftsmanship is not.
