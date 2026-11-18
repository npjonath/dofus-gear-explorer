// jsdom leaves out what ui-sdk's popups, menus and toasts ask the browser for.
// These are the stubs ui-sdk documents for component tests (its testing reference, "jsdom").
import { cleanup } from "@testing-library/react"
import { afterEach } from "vitest"

HTMLElement.prototype.showPopover = function () {}
HTMLElement.prototype.hidePopover = function () {}
Element.prototype.getAnimations = () => []
Element.prototype.scrollIntoView = function () {}
Element.prototype.checkVisibility = () => true
Element.prototype.getClientRects = () => [] as unknown as DOMRectList
document.elementFromPoint = () => null
document.elementsFromPoint = () => []
globalThis.ResizeObserver ??= class {
  observe() {}
  unobserve() {}
  disconnect() {}
}
document.head.append(
  Object.assign(document.createElement("style"), { textContent: "[popover] { display: block !important }" })
)

afterEach(cleanup)
