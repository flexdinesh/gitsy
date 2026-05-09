import type {StatusCategory} from "../status.ts";

export type InkColor =
  | "black"
  | "red"
  | "green"
  | "yellow"
  | "blue"
  | "magenta"
  | "cyan"
  | "white"
  | "gray";

export type CategoryStyle = {
  icon: string;
  label: string;
  color: InkColor;
  bold?: boolean | undefined;
};

export const CATEGORY_STYLES: Record<StatusCategory, CategoryStyle> = {
  modified: {icon: "●", label: "modified", color: "yellow"},
  staged: {icon: "◆", label: "staged", color: "green"},
  untracked: {icon: "+", label: "untracked", color: "green"},
  deleted: {icon: "✖", label: "deleted", color: "red"},
  renamed: {icon: "➜", label: "renamed", color: "magenta"},
  conflict: {icon: "‼", label: "conflict", color: "red", bold: true},
  other: {icon: "•", label: "changed", color: "white"},
};
