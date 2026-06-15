export const normalizeSupportedModelScopesForPlatform = (
  _platform: string,
  _scopes: string[] | undefined,
): string[] => {
  // Per-group supported model scopes are no longer used by any active platform;
  // no per-group scope filtering applies.
  return [];
};
