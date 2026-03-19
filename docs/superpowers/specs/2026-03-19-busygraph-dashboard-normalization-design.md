# BusyGraph Dashboard Normalization Design

## Summary
Normalize the BusyGraph dashboard and quick-stats window into a shared visual system that feels analytical, technical, and beautiful while preserving the product's core strengths: accurate graphs and information-dense presentation.

This is a design and structure pass, not a backend rewrite. Existing endpoints, data models, and metric calculations should remain intact unless a small supporting change is required to present the same data more clearly.

## Problem
The current UI is inconsistent across its two primary surfaces:

- The main dashboard is a generic white-card web dashboard with repeated metric tiles, hard-coded colors, and limited system cohesion.
- The quick-stats window uses a separate gradient-and-glass aesthetic that reads as decorative and disconnected from the main product.
- There is no explicit dual-theme system, no shared design tokens, and no clear hierarchy that ties keyboard, mouse, and video-call data together.

The result is a tool that functions, but does not yet look purpose-built for data-driven users who want a precise and visually compelling personal activity dashboard.

## Users
BusyGraph is primarily for the person who chooses to run it on their own computers. They are typically data-driven, self-directed, and interested in understanding their own computing activity over time.

## Goals
- Make the UI feel like a serious instrument for personal computing telemetry.
- Preserve and improve information density without making the interface feel cluttered.
- Unify the main dashboard and quick-stats window so they share one coherent design system.
- Support both light mode and dark mode intentionally.
- Improve structural consistency so future additions can inherit the same system.

## Non-Goals
- Reworking backend APIs or storage.
- Adding major new product features.
- Converting the dashboard into a sparse marketing-style interface.
- Adding decorative visuals that compete with the charts.

## Design Direction
Adopt a "research notebook" direction:

- Quiet and disciplined rather than flashy.
- Technical and compact rather than oversized and airy.
- Beautiful through precision, typography, alignment, and tone rather than through gradients, blur, or novelty effects.

The interface should feel like a polished personal analysis tool. The charts remain the hero, but the surrounding system should now look intentional and consistent.

## Visual System

### Theme Tokens
Introduce a shared token set for both templates covering:

- App background
- Panel background
- Panel border
- Panel shadow or elevation treatment
- Primary text
- Muted text
- Accent colors for keyboard, mouse, and call data
- Interactive states for controls

The token set should support both light and dark mode with equivalent hierarchy in each theme.

### Typography
- Use a clean, technical sans stack already available on the platform.
- Tighten hierarchy so headings are quieter and metrics are clearer.
- Use tabular numerals where helpful for stats and chart-adjacent values.
- Reduce oversized blue-number styling in favor of more disciplined metric presentation.

### Color
- Use mostly neutral surfaces so the data remains primary.
- Reserve accents for semantic meaning, not decoration.
- Apply the same semantic accents in both light and dark themes.
- Remove the mini window's purple-blue gradient and glassmorphism styling.

## Information Architecture

### Main Dashboard
Preserve the data-rich nature of the current view, but normalize grouping and rhythm:

1. Top heatmaps remain prominent because they are the most distinctive and information-dense views.
2. An overview section follows with the highest-signal summary metrics and range context.
3. Keyboard metrics and charts remain grouped together.
4. Mouse remains compact and summary-first.
5. Video-call metrics and charts remain grouped, followed by the call heatmap.

Section headers should be left-aligned and compact. White-card repetition should be reduced by making layout rhythm, panel hierarchy, and section grouping do more of the work.

### Quick Stats Window
The quick-stats window should become a compact extension of the same system:

- Same tokens
- Same typography
- Same semantic chart colors
- Same surface treatment

It should feel like a condensed instrument panel rather than a stylized widget from a different product.

## Controls and Interaction
- Replace the current generic range buttons with a compact segmented-control treatment.
- Make interactive styling consistent across both themes.
- Keep interactions fast and restrained.
- Avoid decorative motion. Any motion should support clarity rather than style.

## Responsive Behavior
- Keep information density high on wide screens.
- On narrower screens, stack chart pairs and summary rows cleanly.
- Allow large, dense visualizations to remain usable without making the surrounding layout feel broken.
- Make the mini window adaptive rather than dependent on a fixed toy-like composition.

## Implementation Scope

### In Scope
- Introduce shared CSS variables/tokens in the embedded templates.
- Refactor repeated one-off styles into a cohesive surface and spacing system.
- Restyle the main dashboard shell, section headers, cards/panels, and controls.
- Normalize chart container treatments and semantic colors.
- Restyle the mini window to match the dashboard's system.
- Add explicit light/dark theme support using the same token structure.

### Out of Scope
- Replacing Chart.js.
- Rebuilding the data visualizations from scratch.
- Changing server routes or tracker logic beyond what is required to support normalized presentation.

## Implementation Notes
- Keep the existing embedded-HTML architecture.
- Prefer CSS variables and reusable classes over inline styles.
- Reduce hard-coded color literals and unify spacing and border treatments.
- Maintain the existing content order unless a structural move clearly improves scanning.

## Verification
- Review the main dashboard in light mode and dark mode.
- Review the quick-stats window in light mode and dark mode.
- Check that dense views still read clearly at typical laptop widths.
- Run the project's normal Go validation to ensure the templates still build and serve correctly.

## Success Criteria
- The dashboard and mini window clearly feel like the same product.
- The UI looks purpose-built for data-driven personal analysis rather than like a generic template.
- Dense charts remain primary, readable, and visually grounded.
- Light and dark themes both feel intentional.
- Styling is more maintainable because tokens and reusable classes replace scattered one-off rules.
