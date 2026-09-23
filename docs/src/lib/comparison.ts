// Shared model for the comparison table. The Astro frontmatter authors this
// data and the client script reads it back from data attributes, so the types
// live here instead of being duplicated (and drifting) in both places.

export type Tone = "good" | "warn" | "neutral" | "off";

// Linearizes the 2-axis tone system as good < warn < neutral < off.
export const TONE_RANK: Record<Tone, number> = {
  good: 0,
  warn: 1,
  neutral: 2,
  off: 3,
};

export interface CellValue {
  text: string;
  tone: Tone;
  href?: string;
  note?: string;
}

export interface CellItem {
  text: string;
  tone: Tone;
  href: string;
  note?: string;
}

export type Cell = CellValue | { items: CellItem[] };

export interface ToolColumn {
  name: string;
  href: string | null;
  highlight?: boolean;
  // Lifecycle caveat shown as a hover marker beside the tool name in the
  // header. Server-rendered only: left out of the tools data attribute.
  notice?: string;
  // GitHub star count resolved when the docs build runs, null for tools
  // without a repo or when GitHub was unreachable. Fed to the stars
  // order toggle through the tools data attribute.
  stars?: number | null;
  cells: Record<string, Cell>;
}

export interface Capability {
  key: string;
  label: string;
  values: string[];
}

// Tool columns hidden in the default view: raw NixOS primitives + nixos-infect; nixos-anywhere stays visible as the maintained bootstrap representative.
export const DEFAULT_HIDDEN_TOOL_NAMES: readonly string[] = [
  "nixos-rebuild",
  "nixos-install",
  "system.autoUpgrade",
  "nixos-infect",
];

// The text(s) a cell contributes to filtering: item texts for a multi-feature
// cell, the single value otherwise, [] when the cell is absent.
export function cellTexts(cell: Cell | undefined): string[] {
  if (!cell) return [];
  if ("items" in cell) return cell.items.map((item) => item.text);
  return cell.text ? [cell.text] : [];
}

// Sort/filter rank of a capability cell: the best (lowest) item tone for
// multi-feature cells, the tone otherwise, off (worst) when the cell is
// absent. Shared by the SSR dropdown ordering and the client row sort so
// the two can never drift.
export function cellRank(cell: Cell | undefined): number {
  if (!cell) return TONE_RANK.off;
  return "items" in cell
    ? Math.min(...cell.items.map((item) => TONE_RANK[item.tone]))
    : TONE_RANK[cell.tone];
}

// Typed read of a JSON payload embedded in a data attribute. Keeps `any` out
// of the call sites; the payload is server-rendered by this same component.
export function parseJson<T>(raw: string | undefined, label: string): T {
  if (!raw) throw new Error(`missing ${label} data attribute`);
  const parsed: unknown = JSON.parse(raw);
  return parsed as T;
}
