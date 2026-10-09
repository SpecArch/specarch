// requirePermission lets a request through when its session holds the
// permission, and answers 403 otherwise.
export function requirePermission(permission) {
  return (req, res, next) => {
    const granted = req.session?.permissions ?? [];
    if (!granted.includes(permission)) {
      res.status(403).json({ error: 'forbidden' });
      return;
    }
    next();
  };
}
