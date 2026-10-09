export default defineEventHandler((event) => { event.context.session = getCookie(event, "session"); });
