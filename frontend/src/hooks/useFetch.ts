import { useCallback, useEffect, useRef, useState } from "react";
import { get } from "../api/client";

/** Minimal data hook with optional polling; aborts stale responses. */
export function useFetch<T>(path: string | null, pollMs?: number) {
  const [data, setData] = useState<T | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const seq = useRef(0);

  const load = useCallback(async () => {
    if (!path) return;
    const my = ++seq.current;
    try {
      const d = await get<T>(path);
      if (my === seq.current) { setData(d); setError(null); }
    } catch (e) {
      if (my === seq.current) setError((e as Error).message);
    } finally {
      if (my === seq.current) setLoading(false);
    }
  }, [path]);

  useEffect(() => {
    setLoading(true);
    load();
    if (!pollMs) return;
    const t = setInterval(() => { if (!document.hidden) load(); }, pollMs);
    return () => clearInterval(t);
  }, [load, pollMs]);

  return { data, error, loading, reload: load };
}
