// Resolves GitHub star counts at docs build time for the comparison
// table header meta and its stars order toggle. One search API request
// per batch of repo: qualifiers (the search query is capped at 256
// chars); sends Authorization: Bearer only when a GITHUB_TOKEN env var
// is present.
//
// Throws on any failure (unreachable, non-OK, missing repo): a degraded
// fetch fails the docs build, and with it CI, instead of shipping
// age-only header meta.

import { GITHUB_REPOS } from "./github-repos";

interface SearchApiResponse {
  items: {
    full_name: string;
    stargazers_count: number;
  }[];
}

// Accumulate repo: qualifiers into as few batches as the search API's
// 256-char query budget allows.
const batches: string[][] = [[]];
const slugs = [
  ...new Set(Object.values(GITHUB_REPOS).map((repo) => repo.slug)),
];
for (const slug of slugs) {
  const batch = batches[batches.length - 1]!;
  const batchQuery = [...batch, slug]
    .map((batchSlug) => `repo:${batchSlug}`)
    .join("+");
  if (batch.length && batchQuery.length > 256) batches.push([slug]);
  else batch.push(slug);
}

export async function fetchRepoStars(): Promise<Record<string, number>> {
  const headers: Record<string, string> = {
    Accept: "application/vnd.github+json",
    "X-GitHub-Api-Version": "2022-11-28",
  };
  if (process.env.GITHUB_TOKEN)
    headers.Authorization = `Bearer ${process.env.GITHUB_TOKEN}`;

  const itemsByFullName = new Map<string, number>();
  for (const batch of batches) {
    const response = await fetch(
      `https://api.github.com/search/repositories?q=${batch.map((batchSlug) => `repo:${batchSlug}`).join("+")}`,
      { headers },
    );
    if (!response.ok)
      throw new Error(
        `GitHub search API returned ${response.status} for a star batch`,
      );
    const payload = (await response.json()) as SearchApiResponse;
    for (const item of payload.items)
      itemsByFullName.set(item.full_name.toLowerCase(), item.stargazers_count);
  }

  const stars: Record<string, number> = {};
  for (const [toolName, repo] of Object.entries(GITHUB_REPOS)) {
    const starCount = itemsByFullName.get(repo.slug.toLowerCase());
    if (starCount !== undefined) stars[toolName] = starCount;
  }
  if (Object.keys(stars).length < slugs.length) {
    const missing = slugs.filter(
      (slug) => !itemsByFullName.has(slug.toLowerCase()),
    );
    throw new Error(
      `GitHub stars incomplete: ${Object.keys(stars).length}/${slugs.length} resolved (missing: ${missing.join(", ")})`,
    );
  }
  return stars;
}
