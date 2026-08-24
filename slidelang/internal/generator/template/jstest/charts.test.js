// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const { loadModule } = require('./harness.js');

const ALPHA_FRACTION = 128 / 255;
const DEFAULT_COLORS = [
    '#3B82F6', '#10B981', '#F59E0B', '#EF4444',
    '#06B6D4', '#8B5CF6', '#F97316', '#EC4899',
];

test('withAlpha: opaque hex (3/6 digit) gets alpha applied', () => {
    const ctx = loadModule('charts.js');
    assert.equal(ctx.withAlpha('#f00', ALPHA_FRACTION), '#ff000080');
    assert.equal(ctx.withAlpha('#ff0000', ALPHA_FRACTION), '#ff000080');
});

test('withAlpha: hex that already carries alpha (4/8 digit) is never overwritten', () => {
    // The exact H3 regression repro: a theme's own #ff000020 must survive
    // untouched, not become #ff000080.
    const ctx = loadModule('charts.js');
    assert.equal(ctx.withAlpha('#ff000020', ALPHA_FRACTION), '#ff000020');
    assert.equal(ctx.withAlpha('#f008', ALPHA_FRACTION), '#f008');
});

test('withAlpha: rgb()/rgba() without alpha gets ALPHA_FRACTION applied', () => {
    const ctx = loadModule('charts.js');
    assert.equal(ctx.withAlpha('rgb(255, 0, 0)', ALPHA_FRACTION), 'rgba(255, 0, 0, ' + ALPHA_FRACTION + ')');
    // rgba() with only 3 components is a legal alias of rgb() — arity, not
    // function-name prefix, decides whether alpha is already present.
    assert.equal(ctx.withAlpha('rgba(255, 0, 0)', ALPHA_FRACTION), 'rgba(255, 0, 0, ' + ALPHA_FRACTION + ')');
});

test('withAlpha: rgba() that already declares its own alpha is never overwritten', () => {
    // The exact H3 regression repro: rgba(r,g,b,0.1) must survive
    // untouched, not become rgba(r,g,b,0.5019...).
    const ctx = loadModule('charts.js');
    assert.equal(ctx.withAlpha('rgba(10, 20, 30, 0.1)', ALPHA_FRACTION), 'rgba(10, 20, 30, 0.1)');
});

test('withAlpha: hsl()/hsla() without alpha gets ALPHA_FRACTION applied', () => {
    const ctx = loadModule('charts.js');
    assert.equal(ctx.withAlpha('hsl(120, 50%, 50%)', ALPHA_FRACTION), 'hsla(120, 50%, 50%, ' + ALPHA_FRACTION + ')');
});

test('withAlpha: hsla() that already declares its own alpha is never overwritten', () => {
    const ctx = loadModule('charts.js');
    assert.equal(ctx.withAlpha('hsla(120, 50%, 50%, 0.3)', ALPHA_FRACTION), 'hsla(120, 50%, 50%, 0.3)');
});

test('withAlpha: modern slash-alpha syntax is preserved untouched', () => {
    const ctx = loadModule('charts.js');
    assert.equal(ctx.withAlpha('rgb(255 0 0 / 50%)', ALPHA_FRACTION), 'rgb(255 0 0 / 50%)');
});

test('withAlpha: named color falls back to color-mix', () => {
    const ctx = loadModule('charts.js');
    assert.equal(ctx.withAlpha('red', ALPHA_FRACTION), 'color-mix(in srgb, red 50%, transparent)');
});

test('withAlpha: never composes — a color with existing alpha stays exactly as declared regardless of ALPHA_FRACTION passed', () => {
    const ctx = loadModule('charts.js');
    assert.equal(ctx.withAlpha('#ff000020', 0.9), '#ff000020');
    assert.equal(ctx.withAlpha('rgba(1,2,3,0.42)', 0.9), 'rgba(1,2,3,0.42)');
});

test('categoricalColor + withAlpha: defaultColors palette reproduces the exact "...80" suffix a theme with no chart-cat-* tokens always produced', () => {
    const ctx = loadModule('charts.js');
    ctx.SlideLang.metadata = {}; // no themeTokens at all
    DEFAULT_COLORS.forEach((expected, i) => {
        assert.equal(ctx.categoricalColor(i), expected);
        assert.equal(ctx.withAlpha(ctx.categoricalColor(i), ALPHA_FRACTION), expected + '80');
    });
});

test('categoricalColor: a theme chart-cat-* palette overrides defaultColors index-for-index', () => {
    const ctx = loadModule('charts.js');
    ctx.SlideLang.metadata = { themeTokens: { chartCategorical: ['#111111', '#222222'] } };
    assert.equal(ctx.categoricalColor(0), '#111111');
    assert.equal(ctx.categoricalColor(1), '#222222');
    // wraps by module, same as the old defaultColors behavior
    assert.equal(ctx.categoricalColor(2), '#111111');
});

// --- H4/H5: scale dimension detection ---

function baseConfig(type, overrides) {
    return Object.assign(
        {
            type,
            data: { datasets: [{ data: [1, 2, 3] }] },
            options: {},
        },
        overrides
    );
}

test('applyExtensionChartColors: bar chart with only scales.y materializes the missing x scale', () => {
    const ctx = loadModule('charts.js');
    ctx.SlideLang.metadata = { themeTokens: { chart: { 'chart-grid': '#111', 'chart-axis': '#222' } } };
    const config = baseConfig('bar', { options: { scales: { y: { beginAtZero: true } } } });

    ctx.applyExtensionChartColors(config);

    assert.ok(config.options.scales.x, 'expected an "x" scale to be materialized');
    assert.equal(config.options.scales.x.grid.color, '#111');
    assert.equal(config.options.scales.x.ticks.color, '#222');
    assert.equal(config.options.scales.y.grid.color, '#111');
    assert.equal(config.options.scales.y.ticks.color, '#222');
});

// A code-review-flagged correctness bug in an earlier round's "H5 fix":
// it treated a differently-named scale whose declared `axis` matched a
// required dimension (e.g. "revenue": {axis:'y'}) as already covering
// that dimension, so no literal 'y' scale got materialized. That is NOT
// how Chart.js 4.x actually behaves — confirmed empirically against
// Chart.js 4.5.1 by the reviewer: a bar chart with `scales: {revenue:
// {axis:'y'}, x: {...}}` and a dataset that does NOT explicitly set
// yAxisID still gets Chart.js's own default 'y' scale rendered alongside
// "revenue", untamed with Chart.js's own default appearance, because
// nothing told Chart.js's own default-scale materialization to skip it.
// The fix (requiredScaleIds) mirrors Chart.js's real per-dataset
// computation instead of dimension-matching against existing scales.
test('applyExtensionChartColors: a custom-named scale ("revenue": {axis:"y"}) does NOT suppress Chart.js\'s own default "y" — both get materialized and themed', () => {
    const ctx = loadModule('charts.js');
    ctx.SlideLang.metadata = { themeTokens: { chart: { 'chart-grid': '#111', 'chart-axis': '#222' } } };
    const config = baseConfig('bar', {
        // no xAxisID/yAxisID override on the dataset — baseConfig's default
        options: { scales: { revenue: { axis: 'y', beginAtZero: true }, x: { type: 'category' } } },
    });

    ctx.applyExtensionChartColors(config);

    assert.ok(config.options.scales.y, 'expected Chart.js\'s own default "y" to still be materialized (and now themed) alongside "revenue"');
    assert.equal(config.options.scales.y.grid.color, '#111');
    assert.equal(config.options.scales.y.ticks.color, '#222');
    assert.equal(config.options.scales.revenue.grid.color, '#111');
    assert.equal(config.options.scales.x.grid.color, '#111');
    assert.deepEqual(Object.keys(config.options.scales).sort(), ['revenue', 'x', 'y']);
});

test('applyExtensionChartColors: a dataset that explicitly overrides yAxisID to a custom scale suppresses the default "y" (unlike a bare axis:"y" field)', () => {
    const ctx = loadModule('charts.js');
    ctx.SlideLang.metadata = { themeTokens: { chart: { 'chart-grid': '#111' } } };
    const config = baseConfig('bar', {
        data: { datasets: [{ data: [1, 2, 3], yAxisID: 'revenue' }] },
        options: { scales: { revenue: {}, x: {} } },
    });

    ctx.applyExtensionChartColors(config);

    assert.equal(config.options.scales.y, undefined, 'an explicit dataset.yAxisID override, unlike a bare axis field, really does redirect Chart.js away from the default "y"');
    assert.equal(config.options.scales.revenue.grid.color, '#111');
});

test('applyExtensionChartColors: radar chart materializes the radial "r" scale and themes angleLines/pointLabels too', () => {
    const ctx = loadModule('charts.js');
    ctx.SlideLang.metadata = { themeTokens: { chart: { 'chart-grid': '#111', 'chart-axis': '#222' } } };
    const config = baseConfig('radar');

    ctx.applyExtensionChartColors(config);

    assert.ok(config.options.scales.r, 'expected an "r" scale to be materialized for radar');
    assert.equal(config.options.scales.r.grid.color, '#111');
    assert.equal(config.options.scales.r.ticks.color, '#222');
    assert.equal(config.options.scales.r.angleLines.color, '#111');
    assert.equal(config.options.scales.r.pointLabels.color, '#222');
});

test('applyExtensionChartColors: polarArea chart also materializes the radial "r" scale', () => {
    const ctx = loadModule('charts.js');
    ctx.SlideLang.metadata = { themeTokens: { chart: { 'chart-grid': '#111' } } };
    const config = baseConfig('polarArea');

    ctx.applyExtensionChartColors(config);

    assert.ok(config.options.scales.r, 'expected an "r" scale to be materialized for polarArea');
});

// A code-review-flagged correctness bug: Chart.js DOES support a
// per-dataset rAxisID override for radial scales, symmetric with
// xAxisID/yAxisID — confirmed empirically against Chart.js 4.5.1. A radar
// dataset with `rAxisID: "radial"` binds to its own named scale, but
// Chart.js still creates its OWN default 'r' scale alongside it if
// nothing suppresses that requirement — exactly the same "custom-named
// scale doesn't suppress the literal default" shape as the cartesian
// revenue/y case above. An earlier version hard-coded 'r' as the only
// possible radial scale id, so this dataset's real "radial" scale never
// got themed, while Chart.js's OWN untamed default 'r' got materialized
// and themed instead — the exact opposite of the intended chart.
test('applyExtensionChartColors: a radar dataset with rAxisID override themes ITS scale, not just literal "r"', () => {
    const ctx = loadModule('charts.js');
    ctx.SlideLang.metadata = { themeTokens: { chart: { 'chart-grid': '#111', 'chart-axis': '#222' } } };
    const config = baseConfig('radar', {
        data: { datasets: [{ data: [1, 2, 3], rAxisID: 'radial' }] },
    });

    ctx.applyExtensionChartColors(config);

    assert.ok(config.options.scales.radial, 'expected the dataset\'s own named "radial" scale to be materialized');
    assert.equal(config.options.scales.radial.grid.color, '#111');
    assert.equal(config.options.scales.radial.angleLines.color, '#111');
    assert.equal(config.options.scales.radial.pointLabels.color, '#222');
    assert.equal(config.options.scales.r, undefined, 'an explicit dataset.rAxisID override, like yAxisID, really does redirect Chart.js away from the default "r"');
});

test('applyExtensionChartColors: a radar with two datasets, one overriding rAxisID and one using the default, themes BOTH resulting radial scales', () => {
    const ctx = loadModule('charts.js');
    ctx.SlideLang.metadata = { themeTokens: { chart: { 'chart-grid': '#111' } } };
    const config = baseConfig('radar', {
        data: {
            datasets: [
                { data: [1, 2, 3], rAxisID: 'radial' },
                { data: [4, 5, 6] }, // no override — still requires the literal default 'r'
            ],
        },
    });

    ctx.applyExtensionChartColors(config);

    assert.ok(config.options.scales.radial, 'expected the override scale "radial" to be materialized');
    assert.ok(config.options.scales.r, 'expected the literal default "r" to ALSO be materialized for the other dataset');
    assert.equal(config.options.scales.radial.angleLines.color, '#111');
    assert.equal(config.options.scales.r.angleLines.color, '#111');
});

// The generalized model is also dataset.type-aware: a mixed/combo-style
// config where one dataset's own `type` differs from the chart-level
// type gets ITS required dimensions computed from its own type, not
// blindly inherited from the whole chart. This is a synthetic scenario
// (this codebase's buildConfig never actually mixes radial and cartesian
// datasets in one chart), included to lock in the intended generalization
// the reviewer's rAxisID finding pointed at, not just the concrete radar
// case.
test('requiredScaleIds: a dataset.type override is used instead of the chart-level type', () => {
    const ctx = loadModule('charts.js');
    const config = baseConfig('bar', {
        data: { datasets: [{ data: [1, 2, 3], type: 'radar' }] },
    });
    assert.deepEqual(toArray(ctx.requiredScaleIds(config)).sort(), ['r']);
});

test('applyExtensionChartColors: pie chart never gains any scale', () => {
    const ctx = loadModule('charts.js');
    ctx.SlideLang.metadata = { themeTokens: { chart: { 'chart-grid': '#111', 'chart-axis': '#222' } } };
    const config = baseConfig('pie', { options: {} }); // no scales at all, as applyThemeColors leaves it

    ctx.applyExtensionChartColors(config);

    assert.equal(config.options.scales, undefined, 'pie must never gain a scale');
});

test('applyExtensionChartColors: treemap chart never gains any scale (not in CHART_SCALE_DIMENSIONS)', () => {
    const ctx = loadModule('charts.js');
    ctx.SlideLang.metadata = { themeTokens: { chart: { 'chart-grid': '#111' } } };
    const config = baseConfig('treemap', { options: {} });

    ctx.applyExtensionChartColors(config);

    assert.equal(config.options.scales, undefined, 'treemap must never gain a scale');
});

test('applyExtensionChartColors: never overwrites a scale color the author already set explicitly', () => {
    const ctx = loadModule('charts.js');
    ctx.SlideLang.metadata = { themeTokens: { chart: { 'chart-grid': '#111' } } };
    const config = baseConfig('bar', {
        options: { scales: { x: { grid: { color: '#custom' } }, y: {} } },
    });

    ctx.applyExtensionChartColors(config);

    assert.equal(config.options.scales.x.grid.color, '#custom');
    assert.equal(config.options.scales.y.grid.color, '#111');
});

test('applyExtensionChartColors: a theme with no chart tokens at all leaves the config completely untouched', () => {
    const ctx = loadModule('charts.js');
    ctx.SlideLang.metadata = {}; // no themeTokens
    const original = baseConfig('bar', { options: { scales: { y: { beginAtZero: true } } } });
    const config = JSON.parse(JSON.stringify(original));

    ctx.applyExtensionChartColors(config);

    assert.deepEqual(config, original);
});

// --- requiredScaleIds directly ---

// toArray re-materializes a vm-sandbox array (created with the sandbox's
// OWN Array constructor) as a plain array in THIS file's realm — Array.from
// works cross-realm because it only relies on the iterable protocol, not
// on prototype identity. Without this, assert.deepEqual fails with "same
// structure but not reference-equal" even for structurally identical
// arrays, the same cross-realm prototype gotcha datasetAxisIds' own test
// hit earlier (see harness.js's doc comment).
function toArray(sandboxArray) {
    return Array.from(sandboxArray);
}

test('requiredScaleIds: cartesian dataset with no overrides requires literal "x" and "y"', () => {
    const ctx = loadModule('charts.js');
    const config = baseConfig('bar'); // baseConfig's dataset has no xAxisID/yAxisID
    assert.deepEqual(toArray(ctx.requiredScaleIds(config)).sort(), ['x', 'y']);
});

test('requiredScaleIds: a dataset.yAxisID override replaces "y" with the override id', () => {
    const ctx = loadModule('charts.js');
    const config = baseConfig('bar', { data: { datasets: [{ data: [1], yAxisID: 'revenue' }] } });
    assert.deepEqual(toArray(ctx.requiredScaleIds(config)).sort(), ['revenue', 'x']);
});

test('requiredScaleIds: multiple datasets union their required ids', () => {
    const ctx = loadModule('charts.js');
    const config = baseConfig('bar', {
        data: {
            datasets: [
                { data: [1], yAxisID: 'revenue' },
                { data: [2], xAxisID: 'quarter' },
                { data: [3] }, // no override — still requires plain 'x'/'y'
            ],
        },
    });
    assert.deepEqual(toArray(ctx.requiredScaleIds(config)).sort(), ['quarter', 'revenue', 'x', 'y']);
});

test('requiredScaleIds: radial types always require literal "r", regardless of dataset content', () => {
    const ctx = loadModule('charts.js');
    assert.deepEqual(toArray(ctx.requiredScaleIds(baseConfig('radar'))), ['r']);
    assert.deepEqual(toArray(ctx.requiredScaleIds(baseConfig('polarArea', { data: { datasets: [] } }))), ['r']);
});

test('requiredScaleIds: a chart type outside CHART_SCALE_DIMENSIONS requires nothing', () => {
    const ctx = loadModule('charts.js');
    assert.deepEqual(toArray(ctx.requiredScaleIds(baseConfig('pie'))), []);
    assert.deepEqual(toArray(ctx.requiredScaleIds(baseConfig('treemap'))), []);
});
