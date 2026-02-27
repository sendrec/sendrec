import { axe, toHaveNoViolations } from "jest-axe";

expect.extend(toHaveNoViolations);

export async function expectNoA11yViolations(container: HTMLElement) {
  const results = await axe(container, {
    rules: {
      "color-contrast": { enabled: false },
      region: { enabled: false },
    },
  });
  expect(results).toHaveNoViolations();
}
