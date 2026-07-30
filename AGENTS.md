## Learned User Preferences

- Prefer fork-friendly customizations: isolate new features in new files and keep only minimal, clearly marked hooks in upstream files so merges from source stay easy
- When a reference repo or existing implementation is provided, study and match it before inventing a different approach
- Follow through on requested commit, build, and deploy steps rather than stopping after a plan
- After implementing features, always rebuild the Homebox image and deploy (do not leave changes local-only)
- Zebra label printing must use native ZPL generation and send ZPL to the printer; do not treat PNG label output as printable ZPL

## Learned Workspace Facts

- This is a customized Homebox fork; contribution guidelines and Taskfile workflows live in `.github/AGENTS.md`
- Primary deploy target is TrueNAS; compose and agent deploy instructions live under `deploy/truenas/`
- Zebra printing reference is `https://github.com/dylanl321/zebra-label-maker`; default printer is `10.0.1.161:9100`; Homebox sends ZPL directly over TCP and does not require a separate print-server process
