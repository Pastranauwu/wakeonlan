package wemo

// Field order and wording follow a real WeMo Socket: Echo firmware is picky
// about the device description it accepts as a switch.
const setupXML = `<?xml version="1.0"?>
<root xmlns="urn:Belkin:device-1-0">
  <specVersion><major>1</major><minor>0</minor></specVersion>
  <device>
    <deviceType>` + deviceType + `</deviceType>
    <friendlyName>%s</friendlyName>
    <manufacturer>Belkin International Inc.</manufacturer>
    <modelName>Socket</modelName>
    <modelNumber>3.1415</modelNumber>
    <modelDescription>Belkin Plugin Socket 1.0</modelDescription>
    <UDN>%s</UDN>
    <serialNumber>%s</serialNumber>
    <binaryState>%s</binaryState>
    <serviceList>
      <service>
        <serviceType>` + serviceType + `</serviceType>
        <serviceId>urn:Belkin:serviceId:basicevent1</serviceId>
        <controlURL>` + controlPath + `</controlURL>
        <eventSubURL>` + eventPath + `</eventSubURL>
        <SCPDURL>` + eventServicePath + `</SCPDURL>
      </service>
    </serviceList>
  </device>
</root>
`

const eventServiceXML = `<?xml version="1.0"?>
<scpd xmlns="urn:Belkin:service-1-0">
  <actionList>
    <action>
      <name>SetBinaryState</name>
      <argumentList>
        <argument>
          <retval/>
          <name>BinaryState</name>
          <relatedStateVariable>BinaryState</relatedStateVariable>
          <direction>in</direction>
        </argument>
      </argumentList>
    </action>
    <action>
      <name>GetBinaryState</name>
      <argumentList>
        <argument>
          <retval/>
          <name>BinaryState</name>
          <relatedStateVariable>BinaryState</relatedStateVariable>
          <direction>out</direction>
        </argument>
      </argumentList>
    </action>
  </actionList>
  <serviceStateTable>
    <stateVariable sendEvents="yes">
      <name>BinaryState</name>
      <dataType>Boolean</dataType>
      <defaultValue>0</defaultValue>
    </stateVariable>
  </serviceStateTable>
</scpd>
`

const actionResponseXML = `<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
  <s:Body>
    <u:%sResponse xmlns:u="` + serviceType + `"><BinaryState>%s</BinaryState></u:%sResponse>
  </s:Body>
</s:Envelope>
`
