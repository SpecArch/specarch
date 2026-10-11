"use client";

import { SideNav, SideNavItems, SideNavMenu, SideNavMenuItem } from "@carbon/react";
import { usePathname } from "next/navigation";
import { application } from "@/application";
import { visibleMenu } from "./access";
import { useSession } from "./session";
import { say, type Texts } from "./texts";

export interface MenuProps {
  readonly texts: Texts;
}

/** The application's menu, each entry shown only to someone its page opens to. */
export function Menu({ texts }: MenuProps) {
  const pathname = usePathname();
  const session = useSession();
  if (!session) {
    return null;
  }
  return (
    <SideNav aria-label={say(texts, "screens.menu")} expanded isFixedNav isChildOfHeader={false}>
      <SideNavItems>
        {visibleMenu(application.menu, session).map((group) => (
          <SideNavMenu key={group.title} title={say(texts, group.title)} defaultExpanded>
            {group.items.map((item) => (
              <SideNavMenuItem key={item.route} href={item.route} isActive={pathname === item.route}>
                {say(texts, item.title)}
              </SideNavMenuItem>
            ))}
          </SideNavMenu>
        ))}
      </SideNavItems>
    </SideNav>
  );
}
