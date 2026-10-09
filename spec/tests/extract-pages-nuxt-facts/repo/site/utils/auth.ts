export function requirePermission(permission: string): void {
  if (!permission) throw new Error('forbidden');
}
