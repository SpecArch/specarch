// A small stand-in for the browser, enough for Vue's runtime to mount the
// screens under node: elements with attributes, children and listeners,
// text nodes, a document, a window with a location whose hash the router
// reads, the two storages and a stub of fetch the test fills. It is not a
// browser; it is what the smoke test drives the generated screens with.

export class Node {
  constructor() {
    this.childNodes = [];
    this.parentNode = null;
  }
  get nextSibling() {
    if (this.parentNode === null) {
      return null;
    }
    const at = this.parentNode.childNodes.indexOf(this);
    return this.parentNode.childNodes[at + 1] ?? null;
  }
  get firstChild() {
    return this.childNodes[0] ?? null;
  }
  insertBefore(child, anchor) {
    if (child.parentNode !== null) {
      child.parentNode.removeChild(child);
    }
    const at = anchor === null || anchor === undefined ? -1 : this.childNodes.indexOf(anchor);
    if (at < 0) {
      this.childNodes.push(child);
    } else {
      this.childNodes.splice(at, 0, child);
    }
    child.parentNode = this;
    return child;
  }
  appendChild(child) {
    return this.insertBefore(child, null);
  }
  removeChild(child) {
    const at = this.childNodes.indexOf(child);
    if (at >= 0) {
      this.childNodes.splice(at, 1);
      child.parentNode = null;
    }
    return child;
  }
  get textContent() {
    return this.childNodes.map((c) => c.textContent).join("");
  }
  set textContent(text) {
    this.childNodes.forEach((c) => (c.parentNode = null));
    this.childNodes = [];
    if (text !== "") {
      this.appendChild(new Text(String(text)));
    }
  }
}

export class Text extends Node {
  constructor(data) {
    super();
    this.nodeType = 3;
    this.data = String(data);
  }
  get nodeValue() {
    return this.data;
  }
  set nodeValue(v) {
    this.data = String(v);
  }
  get textContent() {
    return this.data;
  }
  set textContent(v) {
    this.data = String(v);
  }
}

export class Comment extends Text {
  constructor(data) {
    super(data);
    this.nodeType = 8;
  }
  get textContent() {
    return "";
  }
}

export class Element extends Node {
  constructor(tag) {
    super();
    this.nodeType = 1;
    this.tagName = tag.toUpperCase();
    this.attributes = new Map();
    this.listeners = new Map();
    this.style = {};
    // The properties Vue sets as DOM properties rather than attributes,
    // present so that "key in el" holds as it does in a browser.
    this.value = "";
    this.disabled = false;
    this.checked = false;
    this.hidden = false;
    this.open = false;
    this.selected = false;
  }
  setAttribute(name, value) {
    this.attributes.set(name, String(value));
  }
  getAttribute(name) {
    return this.attributes.has(name) ? this.attributes.get(name) : null;
  }
  hasAttribute(name) {
    return this.attributes.has(name);
  }
  removeAttribute(name) {
    this.attributes.delete(name);
  }
  get className() {
    return this.getAttribute("class") ?? "";
  }
  set className(v) {
    this.setAttribute("class", v);
  }
  get id() {
    return this.getAttribute("id") ?? "";
  }
  set id(v) {
    this.setAttribute("id", v);
  }
  get innerHTML() {
    return "";
  }
  set innerHTML(_) {
    this.textContent = "";
  }
  addEventListener(type, listener) {
    if (!this.listeners.has(type)) {
      this.listeners.set(type, new Set());
    }
    this.listeners.get(type).add(listener);
  }
  removeEventListener(type, listener) {
    this.listeners.get(type)?.delete(listener);
  }
  /** Fires an event at this element and up its parents, as the browser bubbles one. */
  dispatch(type, extra = {}) {
    const event = { type, target: this, currentTarget: this, defaultPrevented: false, timeStamp: performance.now(), preventDefault() { this.defaultPrevented = true; }, stopPropagation() {}, ...extra };
    for (let at = this; at !== null; at = at.parentNode) {
      event.currentTarget = at;
      for (const listener of at.listeners?.get(type) ?? []) {
        listener(event);
      }
    }
    return event;
  }
  focus() {}
  showModal() {
    this.open = true;
  }
  close() {
    this.open = false;
  }
  get elements() {
    return this.all((e) => e.tagName === "INPUT" || e.tagName === "SELECT" || e.tagName === "TEXTAREA");
  }
  all(predicate, out = []) {
    for (const child of this.childNodes) {
      if (child instanceof Element) {
        if (predicate(child)) {
          out.push(child);
        }
        child.all(predicate, out);
      }
    }
    return out;
  }
  find(predicate) {
    return this.all(predicate)[0];
  }
  byId(id) {
    return this.find((e) => e.getAttribute("id") === id);
  }
  byTag(tag) {
    return this.all((e) => e.tagName === tag.toUpperCase());
  }
}

/** Installs the browser's globals on globalThis, and answers what the test drives them through. */
export function install() {
  const root = new Element("div");
  root.setAttribute("id", "app");
  const documentElement = new Element("html");
  const body = new Element("body");
  documentElement.appendChild(body);
  body.appendChild(root);
  const storage = () => {
    const map = new Map();
    return { getItem: (k) => (map.has(k) ? map.get(k) : null), setItem: (k, v) => map.set(k, String(v)), removeItem: (k) => map.delete(k), clear: () => map.clear() };
  };
  const windowListeners = new Map();
  const fire = (type) => {
    for (const listener of windowListeners.get(type) ?? []) {
      listener({ type });
    }
  };
  let hash = "";
  const location = {
    origin: "http://localhost",
    pathname: "/",
    search: "",
    /** The fragment, as a browser keeps it: with its #, and a change heard as hashchange. */
    get hash() {
      return hash;
    },
    set hash(value) {
      const next = value === "" || value.startsWith("#") ? value : "#" + value;
      if (next !== hash) {
        hash = next;
        queueMicrotask(() => fire("hashchange"));
      }
    },
    replace(url) {
      this.hash = url.startsWith("#") ? url : "";
    },
  };
  const win = {
    location,
    history: { pushState() {}, replaceState() {} },
    addEventListener(type, listener) {
      if (!windowListeners.has(type)) {
        windowListeners.set(type, new Set());
      }
      windowListeners.get(type).add(listener);
    },
    removeEventListener(type, listener) {
      windowListeners.get(type)?.delete(listener);
    },
    matchMedia: () => ({ matches: false, addEventListener() {}, removeEventListener() {} }),
  };
  const doc = {
    documentElement,
    body,
    title: "",
    createElement: (tag) => new Element(tag),
    createElementNS: (_, tag) => new Element(tag),
    createTextNode: (text) => new Text(text),
    createComment: (text) => new Comment(text),
    getElementById: (id) => documentElement.byId(id),
    querySelector: () => null,
  };
  Object.assign(globalThis, { window: win, document: doc, localStorage: storage(), sessionStorage: storage(), Element, HTMLElement: Element, SVGElement: class {}, Blob: globalThis.Blob });
  if (globalThis.location === undefined) {
    Object.defineProperty(globalThis, "location", { get: () => win.location, configurable: true });
  }
  /** Opens a route as a person would: the hash changes and the router hears it. */
  const navigate = async (route) => {
    win.location.hash = "#" + route;
    await settle();
  };
  return { root, window: win, document: doc, navigate };
}

/** Lets every promise and Vue's flush run: a few turns of the microtask queue and one macrotask. */
export async function settle() {
  for (let i = 0; i < 20; i++) {
    await new Promise((resolve) => setTimeout(resolve, 0));
  }
}
