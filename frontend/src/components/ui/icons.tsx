import type { ReactNode, SVGProps } from "react";

/** IconProps are the svg's own props: the size comes from the class, the colour from the text around it. */
export type IconProps = SVGProps<SVGSVGElement>;

/** IconDrawing is an icon as a component, so a button can take any of them. */
export type IconDrawing = (props: IconProps) => ReactNode;

/** drawn makes an icon from strokes on a 24-unit grid: round two-unit lines in the current text colour, no fill. */
function drawn(strokes: ReactNode): IconDrawing {
  return function Icon(props: IconProps) {
    return (
      <svg
        xmlns="http://www.w3.org/2000/svg"
        width={24}
        height={24}
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth={2}
        strokeLinecap="round"
        strokeLinejoin="round"
        {...props}
      >
        {strokes}
      </svg>
    );
  };
}

// Arrows and marks

export const X = drawn(<path d="M6.5 6.5l11 11M17.5 6.5l-11 11" />);
export const Plus = drawn(<path d="M12 5v14M5 12h14" />);
export const Minus = drawn(<path d="M5 12h14" />);
export const Check = drawn(<path d="M5 12.5l4.5 4.5L19 7.5" />);
export const ChevronDown = drawn(<path d="M6.5 9.5 12 15l5.5-5.5" />);
export const ArrowLeft = drawn(<path d="M19 12H5M10.5 6.5 5 12l5.5 5.5" />);
export const ArrowRight = drawn(<path d="M5 12h14M13.5 6.5 19 12l-5.5 5.5" />);
export const Undo = drawn(<path d="M9 5 5 9l4 4M5 9h9.5a5 5 0 0 1 0 10H12" />);
export const Redo = drawn(<path d="m15 5 4 4-4 4M19 9H9.5a5 5 0 0 0 0 10H12" />);
export const RotateBack = drawn(<path d="M4.5 12a7.5 7.5 0 1 0 7.5-7.5M14.5 2 12 4.5 14.5 7" />);
export const Refresh = drawn(<path d="M4.5 12a7.5 7.5 0 0 1 12.8-5.3M17.5 2.5V7H13M19.5 12a7.5 7.5 0 0 1-12.8 5.3M6.5 21.5V17H11" />);
export const LoaderCircle = drawn(<path d="M12 3.5a8.5 8.5 0 1 1-8.5 8.5" />);

// Playback

export const Play = drawn(<path d="M7 4.5v15L19 12Z" />);
export const Pause = drawn(<path d="M8 5v14M16 5v14" />);
export const Stop = drawn(<rect x="5.5" y="5.5" width="13" height="13" rx="1.5" />);
export const Rewind = drawn(<path d="M11 6.5 4.5 12l6.5 5.5ZM19.5 6.5 13 12l6.5 5.5Z" />);
export const FastForward = drawn(<path d="m13 6.5 6.5 5.5-6.5 5.5ZM4.5 6.5 11 12l-6.5 5.5Z" />);
export const Clapperboard = drawn(
  <path d="M3.5 10h17v9a1.5 1.5 0 0 1-1.5 1.5H5A1.5 1.5 0 0 1 3.5 19ZM3.5 5h17v5h-17ZM8.5 5l-2 5M13.5 5l-2 5M18.5 5l-2 5" />,
);
export const Eye = drawn(
  <>
    <path d="M2.5 12C5 7.3 8.3 5 12 5s7 2.3 9.5 7c-2.5 4.7-5.8 7-9.5 7s-7-2.3-9.5-7Z" />
    <circle cx="12" cy="12" r="3" />
  </>,
);
export const EyeOff = drawn(
  <>
    <path d="M2.5 12C5 7.3 8.3 5 12 5s7 2.3 9.5 7c-2.5 4.7-5.8 7-9.5 7s-7-2.3-9.5-7Z" />
    <path d="M3.5 3.5l17 17" />
  </>,
);

// Files and editing

export const FolderOpen = drawn(<path d="M3.5 18.5V5.5A1.5 1.5 0 0 1 5 4h4l2 2.5h6.5A1.5 1.5 0 0 1 19 8v2.5M3.5 18.5l3-8h15l-3 8Z" />);
export const FileDown = drawn(
  <path d="M14 3.5H7A1.5 1.5 0 0 0 5.5 5v14A1.5 1.5 0 0 0 7 20.5h10a1.5 1.5 0 0 0 1.5-1.5V8ZM14 3.5V8h4.5M12 11v6M9.5 14.5 12 17l2.5-2.5" />,
);
export const Document = drawn(
  <path d="M6 3.5h12A1.5 1.5 0 0 1 19.5 5v14a1.5 1.5 0 0 1-1.5 1.5H6A1.5 1.5 0 0 1 4.5 19V5A1.5 1.5 0 0 1 6 3.5ZM8.5 8.5h7M8.5 12h7M8.5 15.5h4" />,
);
export const Save = drawn(
  <path d="M5 3.5h11L19.5 7v12a1.5 1.5 0 0 1-1.5 1.5H5A1.5 1.5 0 0 1 3.5 19V5A1.5 1.5 0 0 1 5 3.5ZM7.5 3.5v4.5h7V3.5M7.5 20.5v-6h9v6" />,
);
export const Download = drawn(<path d="M12 4v11M7 10.5l5 5 5-5M4.5 19.5h15" />);
export const ExternalLink = drawn(
  <path d="M14 4h6v6M20 4l-9 9M18 14v4.5a1.5 1.5 0 0 1-1.5 1.5h-11A1.5 1.5 0 0 1 4 18.5v-11A1.5 1.5 0 0 1 5.5 6H10" />,
);
export const ClipboardCopy = drawn(
  <>
    <path d="M9 4.5H6.5A1.5 1.5 0 0 0 5 6v13.5A1.5 1.5 0 0 0 6.5 21h11a1.5 1.5 0 0 0 1.5-1.5V6a1.5 1.5 0 0 0-1.5-1.5H15M8.5 13.5h7M13 11l2.5 2.5L13 16" />
    <rect x="9" y="3" width="6" height="3" rx="1" />
  </>,
);
export const Pencil = drawn(<path d="m15.5 4.5 4 4L9 19l-5 1 1-5ZM13.5 6.5l4 4" />);
export const PencilLine = drawn(<path d="m14.5 4.5 3.5 3.5L9 17l-4.5 1 1-4.5ZM12.5 6.5l3.5 3.5M13 20.5h7" />);
export const Eraser = drawn(<path d="M3.5 14.5 13 5l6 6-9.5 9.5ZM8.5 9.5l6 6M9.5 20.5H20" />);
export const Trash = drawn(<path d="M4 6.5h16M9.5 6.5V4h5v2.5M6.5 6.5l1 13.5h9l1-13.5M10 10.5v6M14 10.5v6" />);
export const Wand = drawn(<path d="M4.5 19.5 15 9M18 3v3M16.5 4.5h3M20 10v2M19 11h2M9 4v2M8 5h2" />);

// Study

export const BookOpenCheck = drawn(
  <path d="M12 6.5C10 5 7 4.5 3 4.5v14c4 0 7 .5 9 2 2-1.5 5-2 9-2v-14c-4 0-7 .5-9 2ZM12 6.5v14M14.5 12l1.8 1.8 3.2-3.8" />,
);
export const Bookmark = drawn(<path d="M6.5 3.5h11v17L12 16l-5.5 4.5Z" />);
export const Flag = drawn(<path d="M5.5 21V4M5.5 4.5h12L15 9l2.5 4.5h-12" />);
export const CircleDot = drawn(
  <>
    <circle cx="12" cy="12" r="9" />
    <circle cx="12" cy="12" r="3" />
  </>,
);
export const LogOut = drawn(<path d="M9.5 20.5h-4A1.5 1.5 0 0 1 4 19V5a1.5 1.5 0 0 1 1.5-1.5h4M10 12h10.5M16.5 8l4 4-4 4" />);
export const Hourglass = drawn(<path d="M6.5 3h11M6.5 21h11M7.5 3v3.5L12 12l4.5-5.5V3M7.5 21v-3.5L12 12l4.5 5.5V21" />);

// App and computer

export const Settings = drawn(
  <>
    <circle cx="12" cy="12" r="7" />
    <circle cx="12" cy="12" r="2.5" />
    <path d="M12 2.5V5M12 19v2.5M2.5 12H5M19 12h2.5M5.3 5.3 7 7M17 17l1.7 1.7M5.3 18.7 7 17M17 7l1.7-1.7" />
  </>,
);
export const PanelRight = drawn(
  <>
    <rect x="3" y="4" width="18" height="16" rx="2" />
    <path d="M15 4v16" />
  </>,
);
export const Monitor = drawn(
  <>
    <rect x="3" y="4" width="18" height="12" rx="1.5" />
    <path d="M8 20h8M12 16v4" />
  </>,
);
export const Sun = drawn(
  <>
    <circle cx="12" cy="12" r="4" />
    <path d="M12 3v2.5M12 18.5V21M3 12h2.5M18.5 12H21M5.6 5.6l1.8 1.8M16.6 16.6l1.8 1.8M5.6 18.4l1.8-1.8M16.6 7.4l1.8-1.8" />
  </>,
);
export const Moon = drawn(<path d="M19.5 14.5A8 8 0 1 1 9.5 4.5a7.5 7.5 0 0 0 10 10Z" />);
export const Cpu = drawn(
  <>
    <rect x="6" y="6" width="12" height="12" rx="1.5" />
    <rect x="9.5" y="9.5" width="5" height="5" rx=".5" />
    <path d="M9 2.5V6M15 2.5V6M9 18v3.5M15 18v3.5M2.5 9H6M2.5 15H6M18 9h3.5M18 15h3.5" />
  </>,
);
export const Keyboard = drawn(
  <>
    <rect x="2.5" y="6" width="19" height="12" rx="1.5" />
    <path d="M6.5 10h.01M10 10h.01M14 10h.01M17.5 10h.01M6.5 14h.01M17.5 14h.01M9.5 14h5" />
  </>,
);

// Messages

export const CircleHelp = drawn(
  <>
    <circle cx="12" cy="12" r="9" />
    <path d="M9.5 9.5a2.5 2.5 0 1 1 3.5 2.3c-.6.3-1 .8-1 1.5v.5M12 16.5h.01" />
  </>,
);
export const CircleAlert = drawn(
  <>
    <circle cx="12" cy="12" r="9" />
    <path d="M12 7.5V13M12 16.5h.01" />
  </>,
);
export const Info = drawn(
  <>
    <circle cx="12" cy="12" r="9" />
    <path d="M12 11v5.5M12 7.5h.01" />
  </>,
);
export const TriangleAlert = drawn(<path d="M12 3.5 21 19.5H3ZM12 10v4M12 17h.01" />);
