// The routes the branch service registers, each with the permission it checks.
export const routes = [
  { method: "GET", path: "/branches/:branchCode/copies", permission: "copies.read", handler: listCopies },
  { method: "DELETE", path: "/branches/:branchCode/copies", permission: "copies.write", handler: clearShelf },
  { method: "GET", path: "/health", permission: null, handler: health },
  { method: "PUT", path: "/copies/:copyId", permission: "copies.write", handler: saveCopy },
  { method: "PATCH", path: "/copies/:copyId", permission: "copies.write", handler: saveCopy },
  { method: "GET", path: "/files/*", permission: "files.read", handler: serveFile },
  { method: "POST", path: "/copies", permission: "Copies:Write", handler: addCopy },
];
