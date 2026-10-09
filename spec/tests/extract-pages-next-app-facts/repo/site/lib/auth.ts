export async function requirePermission(permission: string): Promise<void> {
  if (!permission) throw new Error('forbidden');
}

export function withPermission<T>(permission: string, handler: (request: Request) => Promise<T>) {
  return async (request: Request) => {
    await requirePermission(permission);
    return handler(request);
  };
}
