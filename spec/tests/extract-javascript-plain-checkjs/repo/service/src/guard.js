/** @param {string} permission */
function allow(permission) {
  return (req, res, next) => (req.user && req.user.can(permission) ? next() : res.sendStatus(403));
}

module.exports = { allow };
