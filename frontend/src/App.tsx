import { lazy, Suspense, useEffect, useState, type ReactNode } from "react";
import { createBrowserRouter, Navigate, useLocation } from "react-router-dom";
import Layout from "./components/Layout";
import { auth, get } from "./api/client";

// Route-level code splitting: the deep-link page ships as its own small chunk,
// so an engineer tapping a Teams/WhatsApp link on mobile loads only what it needs.
const Dashboard = lazy(() => import("./pages/Dashboard"));
const IncidentPage = lazy(() => import("./pages/IncidentPage"));
const DataSources = lazy(() => import("./pages/DataSources"));
const Rules = lazy(() => import("./pages/Rules"));
const Watchers = lazy(() => import("./pages/Watchers"));
const Probes = lazy(() => import("./pages/Probes"));
const Channels = lazy(() => import("./pages/Channels"));
const Login = lazy(() => import("./pages/Login"));

const Fallback = () => <div className="p-8 text-sm text-slate-500">Loading…</div>;
const S = (el: ReactNode) => <Suspense fallback={<Fallback />}>{el}</Suspense>;

/**
 * Zero-trust gate: unauthenticated users go to /login and come back to the exact deep link.
 * With no stored token we ask the API once: if it answers (server runs with AUTH_DISABLED,
 * e.g. the demo stack) the UI proceeds; a 401 sends the user to sign in.
 */
let serverAllowsAnonymous: boolean | null = null;

function RequireAuth({ children }: { children: ReactNode }) {
  const loc = useLocation();
  const hasToken = !!auth.get();
  const [allowed, setAllowed] = useState<boolean | null>(hasToken ? true : serverAllowsAnonymous);

  useEffect(() => {
    if (allowed !== null) return;
    get("/stats")
      .then(() => { serverAllowsAnonymous = true; setAllowed(true); })
      .catch(() => { serverAllowsAnonymous = false; setAllowed(false); });
  }, [allowed]);

  if (allowed === null) return <Fallback />;
  if (!allowed) return <Navigate to="/login" replace state={{ from: loc.pathname + loc.search }} />;
  return <>{children}</>;
}

export const router = createBrowserRouter([
  { path: "/login", element: S(<Login />) },
  {
    element: <RequireAuth><Layout /></RequireAuth>,
    children: [
      { index: true, element: S(<Dashboard />) },
      // Deep link target from Teams / Zoom / WhatsApp / Email: https://platform.local/i/<token>
      { path: "i/:token", element: S(<IncidentPage />) },
      { path: "config/sources", element: S(<DataSources />) },
      { path: "config/rules", element: S(<Rules />) },
      { path: "config/watchers", element: S(<Watchers />) },
      { path: "config/probes", element: S(<Probes />) },
      { path: "config/channels", element: S(<Channels />) },
      { path: "*", element: <Navigate to="/" replace /> },
    ],
  },
]);
