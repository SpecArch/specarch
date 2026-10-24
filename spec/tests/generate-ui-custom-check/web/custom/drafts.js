// The drafts screen, built by hand for this project.
export function show(main, parameters, actions) {
  main.append(document.createTextNode(JSON.stringify({ parameters, actions })));
}
