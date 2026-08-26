# Third-Party Notices

## codex-lb

The Sub2API operator console uses codex-lb as a direct design and source
reference. The application shell, navigation dimensions, status presentation,
control and table treatment, settings composition, login presentation, and
related CSS values in `frontend/src/style.css` and the operator layout and view
components are a Vue port and adaptation based on codex-lb commit
`b311aea760aa639fd96f63bd118f775e9b4a89f9` (`v1.24.0-beta.4`). Sub2API does not
vendor codex-lb's React application. No endorsement by codex-lb or its maintainer
is implied.

The `docs/screenshots/operator-ui/reference-*.jpg` comparison images are resized
copies of codex-lb's `login.jpg`, `dashboard.jpg`, `accounts.jpg`, and
`settings.jpg` documentation screenshots at the commit above.
They remain subject to the codex-lb MIT License below. The matching `before-*`
and `after-*` images contain only Sub2API's local fixture data.

codex-lb is licensed under the MIT License:

```text
MIT License

Copyright (c) 2025 Soju06

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```
