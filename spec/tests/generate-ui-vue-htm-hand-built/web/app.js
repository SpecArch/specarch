import { mount } from "./screens/app.js";
import { page as directory } from "./pages/directory.js";
import { page as invitation } from "./pages/invitation.js";
import { page as memberEdit } from "./pages/member-edit.js";
import { page as memberForm } from "./pages/member-form.js";
import { page as memberView } from "./pages/member-view.js";
import { page as membersList } from "./pages/members-list.js";
import { page as removalForm } from "./pages/removal-form.js";
import { page as removalsInbox } from "./pages/removals-inbox.js";
import { page as signIn } from "./pages/sign-in.js";
import { page as visitsForm } from "./pages/visits-form.js";

/** The pages, by name, each read from its data file. */
const pages = {
  directory: directory,
  invitation: invitation,
  "member-edit": memberEdit,
  "member-form": memberForm,
  "member-view": memberView,
  "members-list": membersList,
  "removal-form": removalForm,
  "removals-inbox": removalsInbox,
  "sign-in": signIn,
  "visits-form": visitsForm,
};

/** The screens the project writes for its custom pages, each imported when its page opens, so one that is missing is said on that page alone. */
const custom = {
};

/** The hooks the project writes for its forms, each module imported when its form opens, one named export per field the settings name, keyed here by the field's name on the wire. */
const hooks = {
  "member-form": () => import("./hooks/member-form.js").then((module) => ({ joinedOn: module.joinedOn })),
};

mount(document.getElementById("app"), { pages, custom, hooks });
