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
  cells: Record<string, Cell>;
}

export interface Capability {
  key: string;
  label: string;
  values: string[];
}

// The text(s) a cell contributes to filtering: item texts for a multi-feature
// cell, the single value otherwise, [] when the cell is absent.
export function cellTexts(cell: Cell | undefined): string[] {
  if (!cell) return [];
  if ("items" in cell) return cell.items.map((item) => item.text);
  return cell.text ? [cell.text] : [];
}

// Typed read of a JSON payload embedded in a data attribute. Keeps `any` out
// of the call sites; the payload is server-rendered by this same component.
export function parseJson<T>(raw: string | undefined, label: string): T {
  if (!raw) throw new Error(`missing ${label} data attribute`);
  const parsed: unknown = JSON.parse(raw);
  return parsed as T;
}
