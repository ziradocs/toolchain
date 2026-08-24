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

test('applyExtensionChartColors: H5 repro — a bar chart already themed via a custom-named y-dimension scale does not also gain a spurious "y"', () => {
    const ctx = loadModule('charts.js');
    ctx.SlideLang.metadata = { themeTokens: { chart: { 'chart-grid': '#111', 'chart-axis': '#222' } } };
    const config = baseConfig('bar', {
        options: { scales: { revenue: { axis: 'y', beginAtZero: true }, x: { type: 'category' } } },
    });

    ctx.applyExtensionChartColors(config);

    assert.equal(config.options.scales.y, undefined, 'expected no spurious "y" scale next to "revenue" (axis: y)');
    assert.equal(config.options.scales.revenue.grid.color, '#111');
    assert.equal(config.options.scales.x.grid.color, '#111');
    // exactly the two the author declared, nothing added
    assert.deepEqual(Object.keys(config.options.scales).sort(), ['revenue', 'x']);
});

test('applyExtensionChartColors: H5 repro via dataset yAxisID reference instead of an explicit axis field', () => {
    const ctx = loadModule('charts.js');
    ctx.SlideLang.metadata = { themeTokens: { chart: { 'chart-grid': '#111' } } };
    const config = baseConfig('bar', {
        data: { datasets: [{ data: [1, 2, 3], yAxisID: 'revenue' }] },
        options: { scales: { revenue: {}, x: {} } },
    });

    ctx.applyExtensionChartColors(config);

    assert.equal(config.options.scales.y, undefined, 'a dataset yAxisID reference must count as covering "y", same as an explicit axis field');
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

// --- scaleDimension / datasetAxisIds directly ---

test('scaleDimension: explicit scale.axis wins over everything else', () => {
    const ctx = loadModule('charts.js');
    assert.equal(ctx.scaleDimension('revenue', { axis: 'y' }, {}), 'y');
});

test('scaleDimension: falls back to dataset xAxisID/yAxisID reference when scale.axis is absent', () => {
    const ctx = loadModule('charts.js');
    assert.equal(ctx.scaleDimension('revenue', {}, { revenue: 'y' }), 'y');
});

test('scaleDimension: falls back to the scale id\'s first letter as the last resort, matching Chart.js\'s own default', () => {
    const ctx = loadModule('charts.js');
    assert.equal(ctx.scaleDimension('revenue', {}, {}), 'r');
    assert.equal(ctx.scaleDimension('x', {}, {}), 'x');
});

test('datasetAxisIds: collects xAxisID/yAxisID across all datasets', () => {
    const ctx = loadModule('charts.js');
    const config = {
        data: {
            datasets: [
                { data: [1], yAxisID: 'revenue' },
                { data: [2], xAxisID: 'quarter' },
                { data: [3] }, // no axis reference — must not throw or add anything
            ],
        },
    };
    // Not assert.deepEqual: the object datasetAxisIds returns is created
    // with the vm sandbox's OWN Object constructor (a different realm
    // than this test file's), so its prototype is never reference-equal
    // to a plain {} literal written here even when structurally
    // identical — deepStrictEqual checks prototype identity too. Compare
    // properties directly instead.
    const result = ctx.datasetAxisIds(config);
    assert.equal(result.revenue, 'y');
    assert.equal(result.quarter, 'x');
    assert.equal(Object.keys(result).length, 2);
});
