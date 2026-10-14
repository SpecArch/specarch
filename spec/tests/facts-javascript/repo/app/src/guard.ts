export function allow(permission: string) {
  return (req: { user?: { can(p: string): boolean } }, res: unknown, next: () => void) => (req.user?.can(permission) ? next() : undefined);
}
