# UnpackUI repository instructions

Maintain extraction, watched folders, recovery and web controls.

Read [.agents/repository-context.md](.agents/repository-context.md) for source locations, invariants and verification commands.

- Preserve archive/path confinement, recursive folder semantics, incomplete extraction recovery and webhook deduplication.
- Use temporary archives and output folders; never point tests or local executable at production media/config.
- Keep the upstream Go module identity unless migration is requested. Linux generation/lint and Windows checks are distinct.

## Repository skills

- For documentation, UI prose and change summaries, read [unslop-unpackui](.agents/skills/unslop-unpackui/SKILL.md).
- For verification, read [verify-unpackui](.agents/skills/verify-unpackui/SKILL.md).
- For implementation, debugging or review, read [change-unpackui](.agents/skills/change-unpackui/SKILL.md).

Load the relevant skill, not the full catalog. These workflows preserve the user's scope and the repository's specific contracts. [Source revisions and licenses](.agents/skill-provenance.md).
