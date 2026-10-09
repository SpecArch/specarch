// The settings the service reads from its environment.
export const port = Number(process.env.PORT ?? '8080');
export const sessionSecret = process.env.SESSION_SECRET;
export const calendarToken = process.env.CALENDAR_TOKEN;
