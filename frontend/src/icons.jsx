import React from 'react';

// Self-contained inline SVG icon set (no external icon library).
function makeIcon(name, children) {
  const Icon = ({ size = 16, color = 'currentColor', strokeWidth = 2, style, ...rest }) => (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke={color}
      strokeWidth={strokeWidth}
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      style={{ flexShrink: 0, ...style }}
      {...rest}
    >
      {children}
    </svg>
  );
  Icon.displayName = name;
  return Icon;
}

export const ArrowLeft = makeIcon('ArrowLeft', <><path d="M19 12H5" /><path d="m12 19-7-7 7-7" /></>);
export const Save = makeIcon('Save', <><path d="M19 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11l5 5v11a2 2 0 0 1-2 2z" /><path d="M17 21v-8H7v8" /><path d="M7 3v5h8" /></>);
export const Play = makeIcon('Play', <polygon points="6 3 20 12 6 21 6 3" />);
export const Filter = makeIcon('Filter', <polygon points="22 3 2 3 10 12.46 10 19 14 21 14 12.46 22 3" />);
export const Trash2 = makeIcon('Trash2', <><path d="M3 6h18" /><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6" /><path d="M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2" /><path d="M10 11v6" /><path d="M14 11v6" /></>);
export const Plus = makeIcon('Plus', <><path d="M5 12h14" /><path d="M12 5v14" /></>);
export const Eye = makeIcon('Eye', <><path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7-10-7-10-7z" /><circle cx="12" cy="12" r="3" /></>);
export const EyeOff = makeIcon('EyeOff', <><path d="M9.88 9.88a3 3 0 1 0 4.24 4.24" /><path d="M10.73 5.08A10.43 10.43 0 0 1 12 5c7 0 10 7 10 7a13.16 13.16 0 0 1-1.67 2.68" /><path d="M6.61 6.61A13.53 13.53 0 0 0 2 12s3 7 10 7a9.74 9.74 0 0 0 5.39-1.61" /><path d="m2 2 20 20" /></>);
export const TerminalSquare = makeIcon('TerminalSquare', <><path d="m7 11 2-2-2-2" /><path d="M11 13h4" /><rect x="3" y="3" width="18" height="18" rx="2" /></>);
export const ChevronDown = makeIcon('ChevronDown', <path d="m6 9 6 6 6-6" />);
export const ChevronRight = makeIcon('ChevronRight', <path d="m9 18 6-6-6-6" />);
export const Menu = makeIcon('Menu', <><path d="M4 6h16" /><path d="M4 12h16" /><path d="M4 18h16" /></>);
export const LayoutDashboard = makeIcon('LayoutDashboard', <><rect x="3" y="3" width="7" height="9" rx="1" /><rect x="14" y="3" width="7" height="5" rx="1" /><rect x="14" y="12" width="7" height="9" rx="1" /><rect x="3" y="16" width="7" height="5" rx="1" /></>);
export const GitMerge = makeIcon('GitMerge', <><circle cx="18" cy="18" r="3" /><circle cx="6" cy="6" r="3" /><path d="M6 21V9a9 9 0 0 0 9 9" /></>);
export const KeyRound = makeIcon('KeyRound', <><path d="M2 18v3c0 .6.4 1 1 1h4v-3h3v-3h2l1.4-1.4a6.5 6.5 0 1 0-4-4Z" /><circle cx="16.5" cy="7.5" r=".5" fill="currentColor" /></>);
export const Users = makeIcon('Users', <><path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2" /><circle cx="9" cy="7" r="4" /><path d="M22 21v-2a4 4 0 0 0-3-3.87" /><path d="M16 3.13a4 4 0 0 1 0 7.75" /></>);
export const LogOut = makeIcon('LogOut', <><path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4" /><path d="m16 17 5-5-5-5" /><path d="M21 12H9" /></>);
export const Activity = makeIcon('Activity', <path d="M22 12h-4l-3 9L9 3l-3 9H2" />);
export const ArrowUpRight = makeIcon('ArrowUpRight', <><path d="M7 17 17 7" /><path d="M7 7h10v10" /></>);
export const ArrowDownRight = makeIcon('ArrowDownRight', <><path d="m7 7 10 10" /><path d="M17 7v10H7" /></>);
export const RefreshCw = makeIcon('RefreshCw', <><path d="M3 12a9 9 0 0 1 9-9 9.75 9.75 0 0 1 6.74 2.74L21 8" /><path d="M21 3v5h-5" /><path d="M21 12a9 9 0 0 1-9 9 9.75 9.75 0 0 1-6.74-2.74L3 16" /><path d="M8 16H3v5" /></>);
export const Layers = makeIcon('Layers', <><path d="m12.83 2.18a2 2 0 0 0-1.66 0L2.6 6.08a1 1 0 0 0 0 1.83l8.58 3.9a2 2 0 0 0 1.66 0l8.58-3.9a1 1 0 0 0 0-1.83Z" /><path d="m22 12.5-8.58 3.9a2 2 0 0 1-1.66 0L2 12.5" /><path d="m22 17.5-8.58 3.9a2 2 0 0 1-1.66 0L2 17.5" /></>);
export const ShieldCheck = makeIcon('ShieldCheck', <><path d="M20 13c0 5-3.5 7.5-7.66 8.95a1 1 0 0 1-.67-.01C7.5 20.5 4 18 4 13V6a1 1 0 0 1 1-1c2 0 4.5-1.2 6.24-2.72a1.17 1.17 0 0 1 1.52 0C14.51 3.81 17 5 19 5a1 1 0 0 1 1 1z" /><path d="m9 12 2 2 4-4" /></>);
export const Zap = makeIcon('Zap', <polygon points="13 2 3 14 12 14 11 22 21 10 12 10 13 2" />);
export const HeartPulse = makeIcon('HeartPulse', <><path d="M19 14c1.49-1.46 3-3.21 3-5.5A5.5 5.5 0 0 0 16.5 3c-1.76 0-3 .5-4.5 2-1.5-1.5-2.74-2-4.5-2A5.5 5.5 0 0 0 2 8.5c0 2.3 1.5 4.05 3 5.5l7 7Z" /><path d="M3.22 12H9.5l.5-1 2 4.5 2-7 1.5 3.5h5.27" /></>);
export const Network = makeIcon('Network', <><rect x="16" y="16" width="6" height="6" rx="1" /><rect x="2" y="16" width="6" height="6" rx="1" /><rect x="9" y="2" width="6" height="6" rx="1" /><path d="M5 16v-3a1 1 0 0 1 1-1h12a1 1 0 0 1 1 1v3" /><path d="M12 12V8" /></>);
export const Award = makeIcon('Award', <><circle cx="12" cy="8" r="6" /><path d="M15.477 12.89 17 22l-5-3-5 3 1.523-9.11" /></>);
export const Copy = makeIcon('Copy', <><rect width="14" height="14" x="8" y="8" rx="2" ry="2" /><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2" /></>);
export const Check = makeIcon('Check', <polyline points="20 6 9 17 4 12" />);
export const Edit2 = makeIcon('Edit2', <><path d="M17 3a2.828 2.828 0 1 1 4 4L7.5 20.5 2 22l1.5-5.5L17 3z" /></>);
export const X = makeIcon('X', <><path d="M18 6 6 18" /><path d="m6 6 12 12" /></>);
export const Scan = makeIcon('Scan', <><path d="M3 7V5a2 2 0 0 1 2-2h2" /><path d="M17 3h2a2 2 0 0 1 2 2v2" /><path d="M21 17v2a2 2 0 0 1-2 2h-2" /><path d="M7 21H5a2 2 0 0 1-2-2v-2" /><circle cx="12" cy="12" r="3" /></>);
export const Server = makeIcon('Server', <><rect width="20" height="8" x="2" y="2" rx="2" ry="2" /><rect width="20" height="8" x="2" y="14" rx="2" ry="2" /><line x1="6" x2="6.01" y1="6" y2="6" /><line x1="6" x2="6.01" y1="18" y2="18" /></>);
export const Lock = makeIcon('Lock', <><rect width="18" height="11" x="3" y="11" rx="2" ry="2" /><path d="M7 11V7a5 5 0 0 1 10 0v4" /></>);



