import { NavLink, Outlet, useNavigate } from "react-router-dom";
import { auth } from "../api/client";

const nav = [
  { to: "/", label: "Incidents", end: true },
  { to: "/config/sources", label: "Data Sources" },
  { to: "/config/rules", label: "Exception Rules" },
  { to: "/config/watchers", label: "Log Watchers" },
  { to: "/config/probes", label: "Probes" },
  { to: "/config/channels", label: "Channels" },
];

export default function Layout() {
  const navigate = useNavigate();
  return (
    <div className="flex min-h-full flex-col md:flex-row">
      <aside className="border-b border-slate-800 bg-slate-950 md:w-56 md:shrink-0 md:border-b-0 md:border-r">
        <div className="flex items-center gap-2 px-4 py-4">
          <span className="grid h-7 w-7 place-items-center rounded-md bg-sky-600 text-xs font-bold text-white">S</span>
          <span className="font-semibold tracking-tight">Sentinel</span>
        </div>
        <nav className="flex gap-1 overflow-x-auto px-2 pb-2 md:flex-col md:pb-4">
          {nav.map((n) => (
            <NavLink
              key={n.to}
              to={n.to}
              end={n.end}
              className={({ isActive }) =>
                `whitespace-nowrap rounded-md px-3 py-2 text-sm ${
                  isActive ? "bg-slate-800 text-white" : "text-slate-400 hover:bg-slate-900 hover:text-slate-200"
                }`
              }
            >
              {n.label}
            </NavLink>
          ))}
        </nav>
        <div className="hidden px-4 py-4 md:block">
          <button className="text-xs text-slate-500 hover:text-slate-300" onClick={() => { auth.clear(); navigate("/login"); }}>
            Sign out
          </button>
        </div>
      </aside>
      <main className="min-w-0 flex-1 p-4 md:p-8">
        <Outlet />
      </main>
    </div>
  );
}
