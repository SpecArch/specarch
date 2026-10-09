export async function requirePermission(permission) {
  if (!permission) throw new Error('forbidden');
}

export function withPermission(permission, handler) {
  return async (req, res) => {
    await requirePermission(permission);
    return handler(req, res);
  };
}
