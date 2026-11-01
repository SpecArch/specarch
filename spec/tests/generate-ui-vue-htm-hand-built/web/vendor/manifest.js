/**
 * The vendor manifest: every file a third-party library ships, committed
 * beside the generated code, with the library's version and licence from
 * the implementation file, the file's path and the integrity index.html
 * loads it with; loaded says whether index.html loads it.
 */
export const manifest = [
  {
    library: "vue",
    version: "3.5.43",
    licence: "MIT",
    file: "vendor/vue.runtime.global.prod.js",
    integrity: "sha384-wRUdzBb9rjxUOB/BX0Nvm1rGocQW6KpyyN7zTekdE+cKxpb0MwSmxzn04ePFtC+O",
    loaded: true,
  },
  {
    library: "htm",
    version: "3.1.1",
    licence: "Apache-2.0",
    file: "vendor/htm.umd.js",
    integrity: "sha384-Ha/rZ1SM6j6GFVMjNonV3LKPW11co4wy3H4IgiMFL/XrLLE68h1euLps89na4wD1",
    loaded: true,
  },
];
