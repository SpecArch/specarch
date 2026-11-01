// A page of headings, paragraphs, images and links under its title, in the
// order a person reads them; it loads and sends nothing.

import { Link } from "./router.js";
import { say } from "./texts.js";
import { h, html } from "./vue.js";

function block(entry, texts, routes) {
  switch (entry.kind) {
    case "heading":
      return h("h" + entry.level, null, say(texts, entry.text));
    case "text":
      return html`<p>${say(texts, entry.text)}</p>`;
    case "image":
      return html`<img src=${entry.path} alt=${say(texts, entry.alternativeText)} />`;
    case "link":
      if (entry.page !== undefined) {
        return html`<p><${Link} to=${routes[entry.page] ?? "/"}>${say(texts, entry.label)}<//></p>`;
      }
      return html`<p><a href=${entry.href}>${say(texts, entry.label)}</a></p>`;
    default:
      return null;
  }
}

export const ContentPage = {
  name: "ContentPage",
  props: { schema: { type: Object, required: true }, texts: { type: Object, required: true }, routes: { type: Object, required: true } },
  setup(props) {
    return () => {
      const { schema, texts, routes } = props;
      return html`<section class="screens-content">
        <h1>${say(texts, schema.title)}</h1>
        ${schema.blocks.map((entry, index) => h("div", { key: index, class: "screens-block" }, block(entry, texts, routes)))}
      </section>`;
    };
  },
};
