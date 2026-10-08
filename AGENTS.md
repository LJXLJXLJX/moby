# Pull requests

When opening a pull request:

- Follow `.github/PULL_REQUEST_TEMPLATE.md` and fill in the applicable sections.
- Do not add `Co-Authored-By` trailers for LLMs or AI agents; reserve them for
  human contributors.
- Add "Created with: <AGENT NAME>" note at the end of the PR body.

## Release note section

If the change is worthy putting in a user-facing release changelog, write a brief entry, ideally one sentence.
Use present tense and imperative tone.
Draw inspiration from past release notes: https://docs.docker.com/engine/release-notes/

## DSec PoC work

Before working on the DSec PoC in this fork, read
`docs/dsec-poc/EXECUTION_GUARDRAILS.md` and the enclosing workspace's `../AGENTS.md`.
They record the current Phase Gate, completed work, scope, and experiment isolation.

This workspace draws selected ideas from arXiv:2609.22978v1 (DeepSeek Elastic
Compute). The full agent reading copy is `../docs/papers/dsec-2609.22978v1/paper.md`,
with official PDF/HTML and LaTeX sources alongside it. The paper is research context;
actual Moby source and the workspace's measured results govern implementation and
claims. Its full 3FS/on-demand loading and resource-management stack is outside the
current local PoC. Do not repeat completed phases or expand scope from the paper.
