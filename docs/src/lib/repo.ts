// Single source of truth for repo metadata shared by the Astro config and header components.
import version from '../../../gen/VERSION?raw';

/** Panix repository URL on GitHub. */
export const REPO_URL = 'https://github.com/mihakrumpestar/panix';

/** Panix version, inlined at build time from gen/VERSION (the same file embedded into the Go binary). */
export const PANIX_VERSION = version.trim();
