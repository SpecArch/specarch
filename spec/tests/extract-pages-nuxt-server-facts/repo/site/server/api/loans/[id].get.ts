export default defineEventHandler((event) => {
  return { id: getRouterParam(event, 'loanId') };
});
