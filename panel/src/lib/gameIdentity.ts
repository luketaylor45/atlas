import {
    Axe,
    Bot,
    Car,
    Code,
    Crosshair,
    Database,
    Factory,
    Gamepad2,
    Mic,
    PawPrint,
    Pickaxe,
    Rocket,
    Shapes,
    Swords,
    Tent,
    type LucideIcon,
} from 'lucide-react';
import type { CSSProperties } from 'react';
import type { Service } from './types';

/** How a server's game is drawn: a category icon and a cover colour. */
export interface GameIdentity {
    /** The game or category, e.g. "Minecraft". */
    label: string;
    icon: LucideIcon;
    /** OKLCH hue of the cover gradient. */
    hue: number;
    /** Which decorative pattern the cover uses, 0-3. */
    pattern: number;
}

interface Category {
    match: RegExp;
    icon: LucideIcon;
    hue: number;
    /** Overrides the name shown, for families with many eggs. */
    label?: string;
}

// Matched against the egg and nest names. Our own icons and colours only;
// no game artwork.
const categories: Category[] = [
    {
        match: /minecraft|\bpaper\b|spigot|bukkit|purpur|\bforge\b|fabric|bungee|velocity|waterfall|sponge|bedrock|nukkit|pocketmine/,
        icon: Pickaxe,
        hue: 150,
        label: 'Minecraft',
    },
    { match: /valheim/, icon: Axe, hue: 60 },
    { match: /palworld/, icon: PawPrint, hue: 190 },
    { match: /garry|\bgmod\b/, icon: Shapes, hue: 230, label: "Garry's Mod" },
    { match: /counter[- ]?strike|\bcs2\b|\bcsgo\b|\btf2\b|team fortress|insurgency|\bsquad\b|\barma\b|battlefield/, icon: Crosshair, hue: 25 },
    { match: /\brust\b|\bark\b|dayz|7 days|seven days|conan|unturned|zomboid|v rising|enshrouded|sons of the forest|survival/, icon: Tent, hue: 40 },
    { match: /terraria|starbound|core keeper|\brpg\b|\bmmo/, icon: Swords, hue: 340 },
    { match: /factorio|satisfactory/, icon: Factory, hue: 85 },
    { match: /space engineers|empyrion|avorion|kerbal|stationeers/, icon: Rocket, hue: 305 },
    { match: /assetto|beamng|racing|\brally\b|truck/, icon: Car, hue: 355 },
    { match: /teamspeak|mumble|\bvoice\b|\bts3\b/, icon: Mic, hue: 320 },
    { match: /discord|\bbots?\b/, icon: Bot, hue: 245 },
    { match: /mysql|mariadb|postgres|redis|mongo|database/, icon: Database, hue: 200 },
    { match: /node\.?js|python|golang|generic|\bapp\b/, icon: Code, hue: 175 },
];

/** A small stable number for a string. */
function hash(text: string): number {
    let value = 2166136261;
    for (let index = 0; index < text.length; index += 1) {
        value ^= text.charCodeAt(index);
        value = Math.imul(value, 16777619);
    }
    return value >>> 0;
}

/** The icon and cover colours for a server, keyed off its egg and nest. */
export function gameIdentity(service: Pick<Service, 'egg'>): GameIdentity {
    const egg = service.egg;
    const nest = egg?.nest;
    const haystack = [egg?.name, nest?.name, nest?.code].filter(Boolean).join(' ').toLowerCase();
    const category = haystack ? categories.find((entry) => entry.match.test(haystack)) : undefined;

    // A sub-nest is usually one game ("Minecraft"); a top-level nest is a
    // group ("Games"), where the egg's own name says more.
    const named = category?.label ?? (nest?.parent_id ? nest.name : egg?.name) ?? 'Server';
    const seed = hash(named.toLowerCase());

    // Unknown games get a hue from their name, kept clear of the indigo accent.
    const generated = (seed % 300 + 300) % 360;
    return {
        label: named,
        icon: category?.icon ?? Gamepad2,
        hue: category?.hue ?? generated,
        pattern: seed % 4,
    };
}

/** The cover gradient for a game hue; its lightness follows the light/dark mode. */
export function coverGradient(hue: number): CSSProperties {
    return {
        backgroundImage: `linear-gradient(135deg, oklch(var(--cover-from) 0.15 ${hue}) 0%, oklch(var(--cover-to) 0.16 ${(hue + 35) % 360}) 100%)`,
    };
}
