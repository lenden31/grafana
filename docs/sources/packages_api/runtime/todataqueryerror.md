+++
# -----------------------------------------------------------------------
# Do not edit this file. It is automatically generated by API Documenter.
# -----------------------------------------------------------------------
title = "toDataQueryError"
keywords = ["grafana","documentation","sdk","@grafana/runtime"]
type = "docs"
disable_edit_link = true
+++

## toDataQueryError() function

### toDataQueryError() function

Convert an object into a DataQueryError -- if this is an HTTP response, it will put the correct values in the error field

<b>Signature</b>

```typescript
export declare function toDataQueryError(err: any): DataQueryError;
```
<b>Import</b>

```typescript
import { toDataQueryError } from '@grafana/runtime';
```
<b>Parameters</b>

|  Parameter | Type | Description |
|  --- | --- | --- |
|  err | <code>any</code> |  |

<b>Returns:</b>

`DataQueryError`
