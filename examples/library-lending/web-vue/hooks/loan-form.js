// The hooks of the loan form, one named export per field the implementation
// file's settings.hooks name, each a function of the loaded record that
// answers the value the field starts with. app.js imports this module when
// the form opens.

/** A copy is lent today, by the desk's own calendar, unless the desk records an earlier day. */
export function lentOn() {
  const today = new Date();
  const month = String(today.getMonth() + 1).padStart(2, "0");
  const day = String(today.getDate()).padStart(2, "0");
  return `${today.getFullYear()}-${month}-${day}`;
}
