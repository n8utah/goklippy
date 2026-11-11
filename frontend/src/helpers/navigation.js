import { Activity, ListFilter, Play, Settings2, Wrench } from 'lucide-react';

/**
 * Every group and page in the app, in rail order (docs/ui-ux-spec.md, "Pages"). The rail,
 * its flyouts, the page tabs, the phone's bottom bar and search all read from here, so a
 * page is added in one place. `parts` are a page's part tabs, each its own URL.
 *
 * Home (/) and Machine › Overview arrive with their pages in build steps 4 and 5; until
 * then / opens Run › Sort and /machine opens Machine › Board.
 */
export const NAV_GROUPS = [
  {
    id: 'run',
    label: 'Run',
    icon: Play,
    pages: [
      { id: 'sort', label: 'Sort', path: '/run/sort', about: 'Start sorting and watch it run' },
      {
        id: 'records',
        label: 'Records',
        path: '/run/records',
        about: 'Every piece, run and incident',
      },
    ],
  },
  {
    id: 'rules',
    label: 'Rules',
    icon: ListFilter,
    pages: [
      {
        id: 'profiles',
        label: 'Profiles',
        path: '/rules/profiles',
        about: 'Which pieces go to which bins',
      },
      {
        id: 'bins',
        label: 'Bins',
        path: '/rules/bins',
        about: 'Bin layout and counts, and where a piece would go',
      },
    ],
  },
  {
    id: 'machine',
    label: 'Machine',
    icon: Wrench,
    pages: [
      {
        id: 'board',
        label: 'Board',
        path: '/machine/board',
        about: 'Connect the board and wire up its slots',
        parts: [
          { id: 'connection', label: 'Connection' },
          { id: 'wiring', label: 'Wiring' },
          { id: 'picture', label: 'Board picture' },
        ],
      },
      {
        id: 'channels',
        label: 'Channels',
        path: '/machine/channels',
        about: 'C1–C4 motors, gearing and speeds, and the chute',
      },
      {
        id: 'chute',
        label: 'Chute',
        path: '/machine/chute',
        about: 'Home and calibrate the chute, test bins',
      },
      {
        id: 'cameras',
        label: 'Cameras',
        path: '/machine/cameras',
        about: 'Which camera watches which channel, zones and pictures',
      },
    ],
  },
  {
    id: 'diagnose',
    label: 'Diagnose',
    icon: Activity,
    pages: [
      {
        id: 'journeys',
        label: 'Piece journeys',
        path: '/diagnose/journeys',
        about: 'Every picture and decision for recent pieces',
      },
    ],
  },
  {
    id: 'system',
    label: 'System',
    icon: Settings2,
    pages: [
      {
        id: 'settings',
        label: 'Settings',
        path: '/system/settings',
        about: 'Every machine setting, by section',
      },
      {
        id: 'account',
        label: 'Account',
        path: '/system/account',
        about: 'Sign in, users and dev tools',
      },
      {
        id: 'ui-kit',
        label: 'UI kit',
        path: '/system/ui-kit',
        about: 'The design system, for building the new pages',
      },
    ],
  },
];

/** Where the app opens until Home exists. */
export const START_PATH = '/run/sort';

/** The group and page a path belongs to, or {} when it's neither. */
export function findLocation(pathname) {
  for (const group of NAV_GROUPS) {
    for (const page of group.pages) {
      if (pathname === page.path || pathname.startsWith(`${page.path}/`)) {
        const rest = pathname.slice(page.path.length + 1).split('/')[0];
        const part = page.parts?.find((p) => p.id === rest) || page.parts?.[0];
        return { group, page, part };
      }
    }
    if (pathname === `/${group.id}` || pathname.startsWith(`/${group.id}/`)) {
      return { group };
    }
  }
  return {};
}

/** Every page, flat, with its group: for search. */
export const ALL_PAGES = NAV_GROUPS.flatMap((group) =>
  group.pages.map((page) => ({ ...page, group })),
);
