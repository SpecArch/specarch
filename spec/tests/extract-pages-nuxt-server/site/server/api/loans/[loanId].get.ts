export default defineEventHandler((event) => ({ id: getRouterParam(event, "loanId") }));
