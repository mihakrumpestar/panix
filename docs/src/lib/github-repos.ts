// Single source of truth mapping comparison-table tool names to their
// GitHub repo: the slug for build-time star resolution and the repo
// creation date (verified against the GitHub API 2026-09-25) hardcoded
// here because it is immutable. Consumed by fetch-repo-stars.ts, which
// resolves stars when the docs build runs.
// Tools absent here are nixpkgs-native primitives with no standalone repo
// and get no header meta.
export interface GitHubRepo {
  slug: string;
  createdAt: string;
}

export const GITHUB_REPOS: Record<string, GitHubRepo> = {
  Panix: { slug: "mihakrumpestar/panix", createdAt: "2025-05-22" },
  "nixos-infect": { slug: "elitak/nixos-infect", createdAt: "2016-04-18" },
  "nixos-anywhere": { slug: "nix-community/nixos-anywhere", createdAt: "2022-11-09", },
  NixOps: { slug: "NixOS/nixops", createdAt: "2011-10-24" },
  Colmena: { slug: "nix-community/colmena", createdAt: "2020-12-16" },
  wire: { slug: "forallsys/wire", createdAt: "2024-07-01" },
  "deploy-rs": { slug: "serokell/deploy-rs", createdAt: "2020-09-28" },
  morph: { slug: "DBCDK/morph", createdAt: "2018-09-24" },
  Nixus: { slug: "Infinisil/nixus", createdAt: "2019-12-22" },
  krops: { slug: "krebs/krops", createdAt: "2019-02-26" },
  lollypops: { slug: "pinpox/lollypops", createdAt: "2022-06-22" },
  nixinate: { slug: "MatthewCroughan/nixinate", createdAt: "2022-01-03" },
  bento: { slug: "rapenne-s/bento", createdAt: "2022-09-08" },
  comin: { slug: "nlewo/comin", createdAt: "2022-12-10" },
  Thymis: { slug: "Thymis-io/thymis", createdAt: "2023-10-01" },
  Clan: { slug: "clan-lol/clan-core", createdAt: "2023-07-11" },
  // The machines feature carries the column; the devenv repo hosts it.
  "devenv machines": { slug: "cachix/devenv", createdAt: "2022-10-22" },
};
