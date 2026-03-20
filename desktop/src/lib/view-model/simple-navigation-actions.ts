import type { SimpleLinkOption } from "@/lib/simple-domain/link-rules";

export function resolveSelectedShareLinkAlias(
  links: SimpleLinkOption[],
  selectedAlias: string | null,
) {
  if (links.length === 0) {
    return null;
  }

  if (selectedAlias && links.some((link) => link.alias === selectedAlias)) {
    return selectedAlias;
  }

  return links[0]?.alias ?? null;
}
