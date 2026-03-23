import { NavMain } from "@/components/nav-main";
import { NavSecondary } from "@/components/nav-secondary";
import {
  Sidebar,
  SidebarContent,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem
} from "@/components/ui/sidebar";
import { GenerateQuote } from "@/quotes";
import {
  BrowserIcon,
  CloudArrowUpIcon,
  GearIcon,
  GithubLogoIcon,
  HouseIcon,
  ListIcon,
  NoteIcon,
  NotebookIcon,
  PersonSimpleThrowIcon,
  SignOutIcon,
  TrafficSignIcon,
  UsersIcon
} from "@phosphor-icons/react";
import * as React from "react";
import { useTranslation } from "react-i18next";
import { TextAnimate } from "./ui/text-animate";
import { ServerStatistics } from "./server-statistics";

export function AppSidebar({ ...props }: React.ComponentProps<typeof Sidebar>) {
  const { t } = useTranslation();

  const userRole = localStorage.getItem("userRole") || "viewer";

  const data = {
    navMain: [
      {
        title: t("sidebar.home"),
        url: "/home",
        icon: HouseIcon
      },
      {
        title: t("sidebar.logs"),
        url: "/logs",
        icon: NotebookIcon
      },
      {
        title: t("sidebar.lists"),
        url: "/blacklist",
        icon: ListIcon,
        items: [
          {
            title: t("sidebar.blacklist"),
            url: "/blacklist"
          },
          {
            title: t("sidebar.whitelist"),
            url: "/whitelist"
          }
        ]
      },
      {
        title: t("sidebar.resolution"),
        url: "/resolution",
        icon: TrafficSignIcon
      },
      {
        title: t("sidebar.prefetch"),
        url: "/prefetch",
        icon: PersonSimpleThrowIcon
      },
      {
        title: t("sidebar.upstream"),
        url: "/upstream",
        icon: CloudArrowUpIcon
      },
      {
        title: t("sidebar.clients"),
        url: "/clients",
        icon: UsersIcon
      },
      ...(userRole === "admin"
        ? [
            {
              title: t("sidebar.users"),
              url: "/users",
              icon: UsersIcon
            }
          ]
        : []),
      {
        title: t("sidebar.settings"),
        url: "/settings",
        icon: GearIcon
      },
      {
        title: t("sidebar.changelog"),
        url: "/changelog",
        icon: NoteIcon
      }
    ],
    navSecondary: [
      {
        title: t("sidebar.website"),
        url: "https://pommee.github.io/goaway",
        icon: BrowserIcon,
        blank: "_blank"
      },
      {
        title: t("sidebar.github"),
        url: "https://github.com/rasty94/goaway",
        icon: GithubLogoIcon,
        blank: "_blank"
      },
      {
        title: t("sidebar.logout"),
        url: "/login",
        icon: SignOutIcon,
        blank: ""
      }
    ]
  };

  return (
    <div className="border-r border-accent">
      <Sidebar variant="inset" {...props}>
        <SidebarHeader>
          <SidebarMenu>
            <SidebarMenuItem>
              <SidebarMenuButton size="lg" asChild>
                <a href="/home">
                  <img src={"/logo.png"} alt={"project-mascot"} width={50} />
                  <div className="grid flex-1 text-left text-lg leading-tight">
                    <span className="truncate font-medium">GoAway</span>
                    <TextAnimate
                      className="truncate text-xs"
                      animation="blurInUp"
                      by="character"
                      once
                    >
                      {GenerateQuote()}
                    </TextAnimate>
                    <span></span>
                  </div>
                </a>
              </SidebarMenuButton>
            </SidebarMenuItem>
          </SidebarMenu>
        </SidebarHeader>
        <ServerStatistics />
        <SidebarContent>
          <NavMain items={data.navMain} />
          <NavSecondary items={data.navSecondary} className="mt-auto" />
        </SidebarContent>
      </Sidebar>
    </div>
  );
}
