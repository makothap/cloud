(window.webpackJsonp=window.webpackJsonp||[]).push([[22],{386:function(t,e,a){"use strict";a.r(e);var r=a(25),n=Object(r.a)({},(function(){var t=this,e=t.$createElement,a=t._self._c||e;return a("ContentSlotsDistributor",{attrs:{"slot-key":t.$parent.slotKey}},[a("h1",{attrs:{id:"working-with-grpc-client"}},[a("a",{staticClass:"header-anchor",attrs:{href:"#working-with-grpc-client"}},[t._v("#")]),t._v(" Working with GRPC Client")]),t._v(" "),a("h2",{attrs:{id:"what-it-grpc"}},[a("a",{staticClass:"header-anchor",attrs:{href:"#what-it-grpc"}},[t._v("#")]),t._v(" What it GRPC")]),t._v(" "),a("p",[t._v("Please follow "),a("a",{attrs:{href:"https://grpc.io/docs/what-is-grpc/introduction/",target:"_blank",rel:"noopener noreferrer"}},[t._v("link"),a("OutboundLink")],1)]),t._v(" "),a("h2",{attrs:{id:"how-to-create-grpc-client-for-grpc-gateway"}},[a("a",{staticClass:"header-anchor",attrs:{href:"#how-to-create-grpc-client-for-grpc-gateway"}},[t._v("#")]),t._v(" How to create GRPC client for grpc-gateway")]),t._v(" "),a("p",[t._v("For creating grpc-client you need to generate a code for your language from proto files, which are stored at "),a("a",{attrs:{href:"https://github.com/plgd-dev/cloud/tree/v2/grpc-gateway/pb",target:"_blank",rel:"noopener noreferrer"}},[t._v("cloud"),a("OutboundLink")],1),t._v(".")]),t._v(" "),a("h3",{attrs:{id:"api"}},[a("a",{staticClass:"header-anchor",attrs:{href:"#api"}},[t._v("#")]),t._v(" API")]),t._v(" "),a("p",[t._v("All requests to service must contains valid access token in "),a("a",{attrs:{href:"https://github.com/grpc/grpc-go/blob/master/Documentation/grpc-auth-support.md#oauth2",target:"_blank",rel:"noopener noreferrer"}},[t._v("grpc metadata"),a("OutboundLink")],1),t._v(".")]),t._v(" "),a("h4",{attrs:{id:"commands"}},[a("a",{staticClass:"header-anchor",attrs:{href:"#commands"}},[t._v("#")]),t._v(" Commands")]),t._v(" "),a("ul",[a("li",[t._v("get devices - list devices")]),t._v(" "),a("li",[t._v("get resource links - list resource links")]),t._v(" "),a("li",[t._v("create resource at device - create resource at the device")]),t._v(" "),a("li",[t._v("retrieve resource from device - get content from the device")]),t._v(" "),a("li",[t._v("retrieve resources values - get resources from the resource shadow")]),t._v(" "),a("li",[t._v("update resources values - update resource at the device")]),t._v(" "),a("li",[t._v("delete resource - delete resource at the device")]),t._v(" "),a("li",[t._v("subscribe for events - provides notification about device registered/unregistered/online/offline, resource published/unpublished/content changed/ ...")]),t._v(" "),a("li",[t._v("get client configuration - provides public configuration for clients(mobile, web, onboarding tool)")])]),t._v(" "),a("h4",{attrs:{id:"contract"}},[a("a",{staticClass:"header-anchor",attrs:{href:"#contract"}},[t._v("#")]),t._v(" Contract")]),t._v(" "),a("ul",[a("li",[a("a",{attrs:{href:"https://github.com/plgd-dev/cloud/blob/v2/grpc-gateway/pb/service.proto",target:"_blank",rel:"noopener noreferrer"}},[t._v("service"),a("OutboundLink")],1)]),t._v(" "),a("li",[a("a",{attrs:{href:"https://github.com/plgd-dev/cloud/blob/v2/grpc-gateway/pb/devices.proto",target:"_blank",rel:"noopener noreferrer"}},[t._v("requests/responses"),a("OutboundLink")],1)]),t._v(" "),a("li",[a("a",{attrs:{href:"https://github.com/plgd-dev/cloud/blob/v2/grpc-gateway/pb/clientConfiguration.proto",target:"_blank",rel:"noopener noreferrer"}},[t._v("client configuration"),a("OutboundLink")],1)])]),t._v(" "),a("h3",{attrs:{id:"go-lang-grpc-client"}},[a("a",{staticClass:"header-anchor",attrs:{href:"#go-lang-grpc-client"}},[t._v("#")]),t._v(" Go-Lang GRPC client")]),t._v(" "),a("h3",{attrs:{id:"creating-client"}},[a("a",{staticClass:"header-anchor",attrs:{href:"#creating-client"}},[t._v("#")]),t._v(" Creating client")]),t._v(" "),a("p",[t._v("Grpc-gateway uses TLS listener, so client must have properly configured TLS.  Here is simple example how to create a grpc client.")]),t._v(" "),a("div",{staticClass:"language-go extra-class"},[a("pre",{pre:!0,attrs:{class:"language-go"}},[a("code",[a("span",{pre:!0,attrs:{class:"token keyword"}},[t._v("import")]),t._v(" "),a("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v("(")]),t._v("\n    "),a("span",{pre:!0,attrs:{class:"token string"}},[t._v('"google.golang.org/grpc"')]),t._v("\n\t"),a("span",{pre:!0,attrs:{class:"token string"}},[t._v('"google.golang.org/grpc/credentials"')]),t._v("\n    "),a("span",{pre:!0,attrs:{class:"token string"}},[t._v('"github.com/plgd-dev/cloud/grpc-gateway/pb"')]),t._v("\n\t"),a("span",{pre:!0,attrs:{class:"token string"}},[t._v('"github.com/plgd-dev/cloud/grpc-gateway/client"')]),t._v("\n"),a("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(")")]),t._v("\n\n    "),a("span",{pre:!0,attrs:{class:"token operator"}},[t._v("...")]),t._v("\n    "),a("span",{pre:!0,attrs:{class:"token comment"}},[t._v("// Create TLS connection to the grpc-gateway.")]),t._v("\n    gwConn"),a("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(",")]),t._v(" err "),a("span",{pre:!0,attrs:{class:"token operator"}},[t._v(":=")]),t._v(" grpc"),a("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(".")]),a("span",{pre:!0,attrs:{class:"token function"}},[t._v("Dial")]),a("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v("(")]),t._v("\n        address"),a("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(",")]),t._v("\n        grpc"),a("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(".")]),a("span",{pre:!0,attrs:{class:"token function"}},[t._v("WithTransportCredentials")]),a("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v("(")]),t._v("credentials"),a("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(".")]),a("span",{pre:!0,attrs:{class:"token function"}},[t._v("NewTLS")]),a("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v("(")]),t._v("tlsConfig"),a("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(")")]),a("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(")")]),a("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(",")]),t._v("\n    "),a("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(")")]),t._v("\n    "),a("span",{pre:!0,attrs:{class:"token keyword"}},[t._v("if")]),t._v(" err "),a("span",{pre:!0,attrs:{class:"token operator"}},[t._v("!=")]),t._v(" "),a("span",{pre:!0,attrs:{class:"token boolean"}},[t._v("nil")]),t._v(" "),a("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v("{")]),t._v("\n        "),a("span",{pre:!0,attrs:{class:"token function"}},[t._v("panic")]),a("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v("(")]),a("span",{pre:!0,attrs:{class:"token string"}},[t._v('"cannot connect to grpc-gateway: "')]),t._v(" "),a("span",{pre:!0,attrs:{class:"token operator"}},[t._v("+")]),t._v(" err"),a("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(".")]),a("span",{pre:!0,attrs:{class:"token function"}},[t._v("Error")]),a("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v("(")]),a("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(")")]),a("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(")")]),t._v("\n    "),a("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v("}")]),t._v("\n    "),a("span",{pre:!0,attrs:{class:"token comment"}},[t._v("// Create basic client which was generated from proto files.")]),t._v("\n    basicClient "),a("span",{pre:!0,attrs:{class:"token operator"}},[t._v(":=")]),t._v(" pb"),a("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(".")]),a("span",{pre:!0,attrs:{class:"token function"}},[t._v("NewGrpcGatewayClient")]),a("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v("(")]),t._v("gwConn"),a("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(")")]),t._v("\n    \n    "),a("span",{pre:!0,attrs:{class:"token comment"}},[t._v("// Create Extended client which provide us more friendly functions.")]),t._v("\n    extendedClient "),a("span",{pre:!0,attrs:{class:"token operator"}},[t._v(":=")]),t._v(" client"),a("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(".")]),a("span",{pre:!0,attrs:{class:"token function"}},[t._v("NewClient")]),a("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v("(")]),t._v("basicClient"),a("span",{pre:!0,attrs:{class:"token punctuation"}},[t._v(")")]),t._v("\n    "),a("span",{pre:!0,attrs:{class:"token operator"}},[t._v("...")]),t._v("\n")])])]),a("h3",{attrs:{id:"using-extended-grpc-client"}},[a("a",{staticClass:"header-anchor",attrs:{href:"#using-extended-grpc-client"}},[t._v("#")]),t._v(" Using extended grpc client")]),t._v(" "),a("p",[t._v("More info in "),a("a",{attrs:{href:"https://pkg.go.dev/github.com/plgd-dev/cloud/grpc-gateway/client",target:"_blank",rel:"noopener noreferrer"}},[t._v("doc"),a("OutboundLink")],1)])])}),[],!1,null,null,null);e.default=n.exports}}]);