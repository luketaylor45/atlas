// Schedules run on five-field cron expressions: minute, hour, day of the
// month, month and day of the week. This module checks them the way the API's
// parser does, builds them from the choices in the schedule form, and puts
// them into words. Everything here is a pure function.

export type RepeatMode = 'daily' | 'weekly' | 'hours' | 'minutes' | 'custom';

/** The choices in the "when" part of the schedule form. */
export interface CronBuilder {
    mode: RepeatMode;
    /** Time of day as 24-hour "HH:MM", for the daily and weekly modes. */
    time: string;
    /** Days for the weekly mode: 0 is Sunday, 6 is Saturday. */
    days: number[];
    /** The gap in hours, for the hours mode. */
    everyHours: number;
    /** The gap in minutes, for the minutes mode. */
    everyMinutes: number;
    /** The expression as typed, for the custom mode. */
    custom: string;
}

export type BuilderField = 'time' | 'days' | 'every' | 'custom';

/** An expression, or what is still wrong with the choices and which one it is about. */
export type BuiltCron = { cron: string; error: null; field: null } | { cron: null; error: string; field: BuilderField };

// Cron counts a step from midnight, or from the top of the hour, so only gaps
// that divide the day or the hour evenly really repeat at that pace.
export const HOUR_GAPS = [1, 2, 3, 4, 6, 8, 12];
export const MINUTE_GAPS = [1, 2, 5, 10, 15, 20, 30];

export const DAY_NAMES = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday'];
const MONTH_NAMES = [
    'January',
    'February',
    'March',
    'April',
    'May',
    'June',
    'July',
    'August',
    'September',
    'October',
    'November',
    'December',
];

/** The days of the week in the order they are shown, Monday first. */
export const WEEK = [1, 2, 3, 4, 5, 6, 0];

export const DEFAULT_BUILDER: CronBuilder = {
    mode: 'daily',
    time: '03:00',
    days: [],
    everyHours: 6,
    everyMinutes: 15,
    custom: '',
};

// ------------------------------------------------------------- Reading

interface FieldRule {
    /** What the field is called in a message. */
    label: string;
    min: number;
    max: number;
    /** How the limits read in a message, when the numbers alone are not clear. */
    limits?: string;
    /** Three-letter names accepted in place of numbers; the first stands for `min`. */
    names?: string[];
}

const shortNames = (names: string[]) => names.map((name) => name.slice(0, 3).toLowerCase());

const FIELD_RULES: FieldRule[] = [
    { label: 'minute', min: 0, max: 59 },
    { label: 'hour', min: 0, max: 23 },
    { label: 'day of the month', min: 1, max: 31 },
    { label: 'month', min: 1, max: 12, names: shortNames(MONTH_NAMES) },
    { label: 'day of the week', min: 0, max: 6, limits: '0 (Sunday) and 6 (Saturday)', names: shortNames(DAY_NAMES) },
];

const FIVE_FIELDS = 'Enter five values separated by spaces: minute, hour, day of the month, month and day of the week.';

interface CronField {
    /** Every value the field matches, lowest first. */
    values: number[];
    /** True for "*" or "?": the field narrows nothing down. */
    any: boolean;
}

function readValue(token: string, rule: FieldRule): number | null {
    const named = rule.names ? rule.names.indexOf(token.toLowerCase()) : -1;
    if (named >= 0) return rule.min + named;
    return /^\d+$/.test(token) ? Number(token) : null;
}

/** Reads one field, or returns what is wrong with it. */
function readField(text: string, rule: FieldRule): CronField | string {
    const matched = new Set<number>();
    let any = false;

    for (const part of text.split(',')) {
        const invalid = part === '' ? `The ${rule.label} has a comma with nothing beside it.` : `"${part}" is not a valid ${rule.label}.`;
        const pieces = part.split('/');
        if (pieces.length > 2 || pieces[0] === '') return invalid;

        let step = 1;
        if (pieces.length === 2) {
            if (!/^\d+$/.test(pieces[1])) return invalid;
            step = Number(pieces[1]);
            if (step === 0) return `In "${part}", the number after the slash must be 1 or more.`;
        }

        let start = rule.min;
        let end = rule.max;
        if (pieces[0] === '*' || pieces[0] === '?') {
            if (step === 1) any = true;
        } else {
            const bounds = pieces[0].split('-');
            const low = readValue(bounds[0], rule);
            const high = bounds.length === 1 ? low : readValue(bounds[1], rule);
            if (bounds.length > 2 || low === null || high === null) return invalid;

            start = low;
            // "5/15" means every 15 from 5 to the end of the range.
            end = bounds.length === 1 && pieces.length === 2 ? rule.max : high;
            if (start < rule.min || start > rule.max || end > rule.max) {
                return `The ${rule.label} must be between ${rule.limits ?? `${rule.min} and ${rule.max}`}.`;
            }
            if (start > end) return `In "${part}", the range must run from the lower value to the higher one.`;
        }

        for (let value = start; value <= end; value += step) matched.add(value);
    }

    return { values: [...matched].sort((a, b) => a - b), any };
}

/** Reads all five fields, or returns what is wrong with the expression. */
function readCron(expression: string): CronField[] | string {
    const parts = expression.trim().split(/\s+/);
    if (parts.length !== 5 || parts[0] === '') return FIVE_FIELDS;

    const fields: CronField[] = [];
    for (const [index, part] of parts.entries()) {
        const field = readField(part, FIELD_RULES[index]);
        if (typeof field === 'string') return field;
        fields.push(field);
    }
    return fields;
}

/** The expression with single spaces between its fields, the way the API stores it. */
export function normaliseCron(expression: string): string {
    return expression.trim().split(/\s+/).join(' ');
}

/** What is wrong with an expression, in a sentence, or null when it is fine. */
export function cronError(expression: string): string | null {
    const fields = readCron(expression);
    return typeof fields === 'string' ? fields : null;
}

// ------------------------------------------------------------ Building

const TIME_OF_DAY = /^([01]?\d|2[0-3]):([0-5]\d)$/;

const pad = (value: number) => String(value).padStart(2, '0');
const clock = (hour: number, minute: number) => `${pad(hour)}:${pad(minute)}`;

const built = (cron: string): BuiltCron => ({ cron, error: null, field: null });
const problem = (field: BuilderField, error: string): BuiltCron => ({ cron: null, error, field });

/** Turns the form's choices into an expression. */
export function buildCron(builder: CronBuilder): BuiltCron {
    switch (builder.mode) {
        case 'daily':
        case 'weekly': {
            const time = TIME_OF_DAY.exec(builder.time.trim());
            const days = WEEK.filter((day) => builder.days.includes(day)).sort((a, b) => a - b);
            if (builder.mode === 'weekly' && days.length === 0) return problem('days', 'Choose at least one day.');
            if (!time) return problem('time', 'Enter a time of day, for example 03:00.');
            return built(`${Number(time[2])} ${Number(time[1])} * * ${builder.mode === 'weekly' ? days.join(',') : '*'}`);
        }
        case 'hours':
            if (!HOUR_GAPS.includes(builder.everyHours)) return problem('every', 'Choose how often it runs.');
            return built(builder.everyHours === 1 ? '0 * * * *' : `0 */${builder.everyHours} * * *`);
        case 'minutes':
            if (!MINUTE_GAPS.includes(builder.everyMinutes)) return problem('every', 'Choose how often it runs.');
            return built(builder.everyMinutes === 1 ? '* * * * *' : `*/${builder.everyMinutes} * * * *`);
        default: {
            const error = cronError(builder.custom);
            return error ? problem('custom', error) : built(normaliseCron(builder.custom));
        }
    }
}

/** The gap between values that start at zero and divide `size` evenly (0, 15, 30, 45 of 60 gives 15), or null. */
function evenGap(values: number[], size: number): number | null {
    const gap = size / values.length;
    if (values.length < 2 || !Number.isInteger(gap)) return null;
    return values.every((value, index) => value === index * gap) ? gap : null;
}

/**
 * The form choices that produce an existing expression. One that fits none of
 * the simpler modes comes back as custom, and so does one that cannot be read.
 */
export function builderFromCron(expression: string): CronBuilder {
    const custom: CronBuilder = { ...DEFAULT_BUILDER, mode: 'custom', custom: normaliseCron(expression) };
    const fields = readCron(expression);
    if (typeof fields === 'string') return custom;

    const [minute, hour, dayOfMonth, month, dayOfWeek] = fields;
    if (!dayOfMonth.any || month.values.length !== 12) return custom;

    if (minute.values.length === 1 && hour.values.length === 1) {
        const time = clock(hour.values[0], minute.values[0]);
        return dayOfWeek.any ? { ...custom, mode: 'daily', time } : { ...custom, mode: 'weekly', time, days: dayOfWeek.values };
    }
    if (!dayOfWeek.any) return custom;

    const everyHours = evenGap(hour.values, 24);
    if (minute.values.length === 1 && minute.values[0] === 0 && everyHours !== null && HOUR_GAPS.includes(everyHours)) {
        return { ...custom, mode: 'hours', everyHours };
    }
    const everyMinutes = evenGap(minute.values, 60);
    if (hour.values.length === 24 && everyMinutes !== null && MINUTE_GAPS.includes(everyMinutes)) {
        return { ...custom, mode: 'minutes', everyMinutes };
    }
    return custom;
}

// ---------------------------------------------------------- Describing

const count = (value: number, unit: string) => `${value} ${unit}${value === 1 ? '' : 's'}`;
const lowerFirst = (text: string) => text.charAt(0).toLowerCase() + text.slice(1);

/** "a", "a and b", "a, b and c". */
function sentenceList(items: string[]): string {
    if (items.length <= 1) return items.join('');
    return `${items.slice(0, -1).join(', ')} and ${items[items.length - 1]}`;
}

/**
 * Puts values into words, shortening what would be a long list: a run becomes
 * "1 to 5" and a long even pattern "1, 3, 5 and so on up to 31".
 */
function valueList(values: number[], name: (value: number) => string = String): string {
    const step = values.length > 1 ? values[1] - values[0] : 0;
    if (values.length > 6 && step > 1 && values.every((value, index) => value === values[0] + index * step)) {
        return `${values.slice(0, 3).map(name).join(', ')} and so on up to ${name(values[values.length - 1])}`;
    }

    const parts: string[] = [];
    let index = 0;
    while (index < values.length) {
        let end = index;
        while (end + 1 < values.length && values[end + 1] === values[end] + 1) end += 1;
        if (end - index >= 2) {
            parts.push(`${name(values[index])} to ${name(values[end])}`);
        } else {
            for (let each = index; each <= end; each += 1) parts.push(name(values[each]));
        }
        index = end + 1;
    }
    return sentenceList(parts);
}

/** Days of the week in words, Monday first: "Monday and Friday", or "weekday" for Monday to Friday. */
function weekDayWords(values: number[]): string {
    const days = WEEK.filter((day) => values.includes(day));
    const weekdays = days.length === 5 && !values.includes(0) && !values.includes(6);
    return weekdays ? 'weekday' : sentenceList(days.map((day) => DAY_NAMES[day]));
}

/** The days it runs on: "on Monday and Friday", "on day 1 of the month", or nothing for every day. */
function dayPhrase(dayOfMonth: CronField, dayOfWeek: CronField): string {
    const everyMonthDay = dayOfMonth.values.length === 31;
    const everyWeekDay = dayOfWeek.values.length === 7;
    // With both day fields narrowed down, cron runs when either one matches.
    const either = !dayOfMonth.any && !dayOfWeek.any;
    if (either ? everyMonthDay || everyWeekDay : everyMonthDay && everyWeekDay) return '';

    const words = weekDayWords(dayOfWeek.values);
    return [
        everyMonthDay ? '' : `on ${dayOfMonth.values.length === 1 ? 'day' : 'days'} ${valueList(dayOfMonth.values)} of the month`,
        everyWeekDay ? '' : `on ${words === 'weekday' ? 'weekdays' : words}`,
    ]
        .filter(Boolean)
        .join(', and also ');
}

/** The time it runs at. `clock` marks plain times of day ("03:00"), which read best after the days. */
function timePhrase(minute: CronField, hour: CronField): { text: string; clock: boolean } {
    const oneMinute = minute.values.length === 1 ? minute.values[0] : null;
    const minuteGap = evenGap(minute.values, 60);
    const repeating = (text: string) => ({ text, clock: false });

    let minutes: string;
    if (minuteGap === 1) minutes = 'Every minute';
    else if (minuteGap !== null) minutes = `Every ${minuteGap} minutes`;
    else if (oneMinute === 0) minutes = 'On the hour';
    else minutes = `At ${valueList(minute.values)} ${oneMinute === 1 ? 'minute' : 'minutes'} past the hour`;

    if (hour.values.length === 24) {
        if (minuteGap !== null) return repeating(minutes);
        return repeating(oneMinute === 0 ? 'Every hour, on the hour' : `Every hour, ${lowerFirst(minutes)}`);
    }

    const hourGap = evenGap(hour.values, 24);
    if (oneMinute !== null && hourGap !== null) {
        const times = hour.values.map((value) => clock(value, oneMinute));
        return repeating(`Every ${hourGap} hours (${times.length <= 4 ? sentenceList(times) : `${times.slice(0, 3).join(', ')} and so on`})`);
    }
    if (minute.values.length * hour.values.length <= 6) {
        const times = hour.values.flatMap((value) => minute.values.map((each) => clock(value, each)));
        return { text: sentenceList(times), clock: true };
    }

    const first = hour.values[0];
    const last = hour.values[hour.values.length - 1];
    if (last - first !== hour.values.length - 1) return repeating(`${minutes} during the hours ${valueList(hour.values)}`);
    if (oneMinute !== null) return repeating(`Every hour from ${clock(first, oneMinute)} to ${clock(last, oneMinute)}`);
    return repeating(`${minutes} from ${clock(first, 0)} to ${clock(last, 59)}`);
}

/** An expression in plain words ("Every Monday and Friday at 03:00"), or null when it cannot be read. */
export function describeCron(expression: string): string | null {
    const fields = readCron(expression);
    if (typeof fields === 'string') return null;
    const [minute, hour, dayOfMonth, month, dayOfWeek] = fields;

    const days = dayPhrase(dayOfMonth, dayOfWeek);
    const months = month.values.length === 12 ? '' : `in ${valueList(month.values, (value) => MONTH_NAMES[value - 1])}`;
    const where = [days, months].filter(Boolean).join(' ');

    const time = timePhrase(minute, hour);
    if (!time.clock) return where ? `${time.text}, ${where}` : time.text;
    if (!where) return `Every day at ${time.text}`;
    if (!months && dayOfMonth.any) return `Every ${weekDayWords(dayOfWeek.values)} at ${time.text}`;
    if (!days) return `Every day ${months} at ${time.text}`;
    return `At ${time.text} ${where}`;
}

// ----------------------------------------------------------- Time zone

/** An offset in seconds east of UTC as "UTC+2", "UTC-3:30" or "UTC+0". */
export function formatUtcOffset(seconds: number): string {
    const minutes = Math.round(Math.abs(seconds) / 60);
    const rest = minutes % 60;
    return `UTC${seconds < 0 && minutes > 0 ? '-' : '+'}${Math.floor(minutes / 60)}${rest ? `:${pad(rest)}` : ''}`;
}

/**
 * Names the time zone schedules run in, from what the API reports about it:
 * "Europe/London, UTC+1", "CEST, UTC+2" or just "UTC".
 */
export function describeZone(name: string, abbreviation: string, offsetSeconds: number): string {
    // A zone without a name of its own is reported as "Local", and one without an abbreviation as its offset.
    const label = name && name !== 'Local' ? name : /^[a-z]/i.test(abbreviation) ? abbreviation : '';
    if (label === 'UTC' || (!label && Math.round(offsetSeconds / 60) === 0)) return 'UTC';
    const offset = formatUtcOffset(offsetSeconds);
    return label ? `${label}, ${offset}` : offset;
}

/** How a zone's clock compares with the viewer's: "the same as yours" or "2 hours ahead of yours". */
export function compareZone(zoneOffsetSeconds: number, viewerOffsetSeconds: number): string {
    const minutes = Math.round((zoneOffsetSeconds - viewerOffsetSeconds) / 60);
    if (minutes === 0) return 'the same as yours';
    const hours = Math.floor(Math.abs(minutes) / 60);
    const rest = Math.abs(minutes) % 60;
    const gap = [hours ? count(hours, 'hour') : '', rest ? count(rest, 'minute') : ''].filter(Boolean).join(' ');
    return `${gap} ${minutes > 0 ? 'ahead of' : 'behind'} yours`;
}
