import { ChevronRight, Home } from 'lucide-react';

const crumb =
    'inline-flex items-center gap-1.5 rounded-control px-1.5 py-1 text-sm text-muted transition-colors hover:bg-secondary hover:text-foreground';
const currentCrumb = 'inline-flex items-center gap-1.5 px-1.5 py-1 text-sm font-medium text-foreground';

/** The current folder as clickable segments, starting from the server's root. */
export default function Breadcrumbs({ path, onNavigate }: { path: string; onNavigate: (path: string) => void }) {
    const parts = path.split('/').filter(Boolean);

    return (
        <nav aria-label="Folder path" className="min-w-0">
            <ol className="flex flex-wrap items-center gap-0.5">
                <li>
                    {parts.length === 0 ? (
                        <span className={currentCrumb} aria-current="page">
                            <Home size={16} aria-hidden />
                            Home
                        </span>
                    ) : (
                        <button type="button" className={crumb} onClick={() => onNavigate('')}>
                            <Home size={16} aria-hidden />
                            Home
                        </button>
                    )}
                </li>
                {parts.map((part, index) => {
                    const target = parts.slice(0, index + 1).join('/');
                    const last = index === parts.length - 1;
                    return (
                        <li key={target} className="flex min-w-0 items-center gap-0.5">
                            <ChevronRight size={16} className="shrink-0 text-subtle" aria-hidden />
                            {last ? (
                                <span className={`${currentCrumb} break-all`} aria-current="page">
                                    {part}
                                </span>
                            ) : (
                                <button type="button" className={`${crumb} break-all text-left`} onClick={() => onNavigate(target)}>
                                    {part}
                                </button>
                            )}
                        </li>
                    );
                })}
            </ol>
        </nav>
    );
}
