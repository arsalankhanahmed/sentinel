import { lazy, Suspense, type ReactNode } from "react";
import { createBrowserRouter, Navigate, useLocation } from "react-router-dom";
import Layout from "./components/Layout";
import { auth } from "./api/client";

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

/** Zero-trust gate: unauthenticated users go to /login and come back to the exact deep link. */
function RequireAuth({ children }: { children: ReactNode }) {
  const loc = useLocation();
  if (!auth.get() && import.meta.env.VITE_AUTH_DISABLED !== "true") {
    return <Navigate to="/login" replace state={{ from: loc.pathname + loc.search }} />;
  }
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
