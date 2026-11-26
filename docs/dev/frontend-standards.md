# Frontend Engineering Standards

Every rule below states whether it is enforced by an automated tool or by code review.

## Stack and Type Safety

- **TypeScript strict mode**: `strict`, `noUnusedLocals`, `noUnusedParameters`, and `noFallthroughCasesInSwitch` must remain enabled in `web/tsconfig.json`.
  - *Enforced by*: `pnpm --dir web typecheck` (`tsc --noEmit`).
- **Linting**: Frontend source under `web/src` must pass `oxlint` with zero warnings or errors.
  - *Enforced by*: `pnpm --dir web lint`.
- **Production build**: `tsc -b && vite build` must succeed and emit static assets to `web/dist/`.
  - *Enforced by*: `pnpm --dir web build`.

## Internationalization and Copy

- **English default & i18n readiness**: Write all code, comments, and default UI text in English. Do not hardcode user-facing strings directly in JSX; store them in a messages dictionary / `t()` helper.
  - *Enforced by*: Review.

## Styling and Theme (Dose)

- **CSS custom properties**: Use the Dose theme tokens in `web/src/index.css` (`--color-paper`, `--color-ink`, `--color-accent`, `--font-display`, `--font-body`, `--font-mono`). Do not scatter literal colors or font families outside `:root`.
  - *Enforced by*: Review.
- **Motion and focus**: Every interactive control must define `:focus-visible`, avoid `transition: all`, and respect `prefers-reduced-motion: reduce`.
  - *Enforced by*: Review.
- **Overflow safety**: Keep `overflow-x: clip` on `html` and `body`.
  - *Enforced by*: Review.
