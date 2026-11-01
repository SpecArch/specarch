// The choice of the language the screens are shown in, kept in the
// browser's local storage so it outlasts the session; the screens are
// drawn again in it. Nothing is drawn when the screens have one language.

import { choose, language, nameOf } from "./language.js";
import { languages, say } from "./texts.js";
import { html } from "./vue.js";

export const LanguageChoice = {
  name: "LanguageChoice",
  props: { texts: { type: Object, required: true } },
  setup(props) {
    return () => {
      if (languages.length < 2) {
        return null;
      }
      return html`<label class="screens-language">${say(props.texts, "screens.language")}
        <select id="screens-language" value=${language.value} onChange=${(event) => choose(event.target.value)}>
          ${languages.map((code) => html`<option key=${code} value=${code} lang=${code}>${nameOf(code)}</option>`)}
        </select>
      </label>`;
    };
  },
};
