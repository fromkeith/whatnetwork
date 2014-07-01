/*
 * Copyright (c) 2013, fromkeith
 * All rights reserved.
 * 
 * Redistribution and use in source and binary forms, with or without modification,
 * are permitted provided that the following conditions are met:
 * 
 * * Redistributions of source code must retain the above copyright notice, this
 *   list of conditions and the following disclaimer.
 * 
 * * Redistributions in binary form must reproduce the above copyright notice, this
 *   list of conditions and the following disclaimer in the documentation and/or
 *   other materials provided with the distribution.
 * 
 * * Neither the name of the fromkeith nor the names of its
 *   contributors may be used to endorse or promote products derived from
 *   this software without specific prior written permission.
 * 
 * THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND
 * ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
 * WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
 * DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR
 * ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES
 * (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES;
 * LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON
 * ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
 * (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
 * SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
 */

/*
    whatnetwork
    ===========
    A simply library to test what the basic reason for a http request failing is.

    example
    =======
    func main() {
        aa, ee := CheckNetworkConnection()
        log.Println("CheckNetworkConnection ", aa, ee)
        //return
        theUrl, _ := url.Parse("http://192.168.1.126:8282")
        hreq := http.Request {
            URL: theUrl,
            Header: http.Header{},
            Proto:      "HTTP/1.1",
            ProtoMajor: 1,
            ProtoMinor: 1,
            Close: true,
        }
        _, err := http.DefaultClient.Do(&hreq)
        if err != nil {
            log.Println("BasicError: ", ExtractBasicError(err))
        }
    }


*/
package whatnetwork


import (
    "net/http"
    "log"
    "net/url"
    "net"
    "os"
    "io"
    "strings"
)

type BasicErrorType string

const (
    BasicError_CantResolveHost      BasicErrorType = "CantResolveHost"
    BasicError_CantFindHost         BasicErrorType = "CantFindHost"
    BasicError_CantConnectToHost    BasicErrorType = "CantConnectToHost"
    BasicError_UnexpectedEof        BasicErrorType = "UnexpectedEof"
    BasicError_BadDataReceived      BasicErrorType = "BadDataReceived"
    BasicError_Unknown              BasicErrorType = "Unknown"
)

type BasicError struct {
    BasicError      BasicErrorType
    Cause           error
}

func (b BasicError) Error() string {
    return string(b.BasicError)
}

type ConnectionStatus string

const (
    ConnectionStatus_NoInterfaces   ConnectionStatus = "NoInterfaces"
    ConnectionStatus_NoInterfacesUp ConnectionStatus = "NoInterfacesUp"
    ConnectionStatus_NoNonLoopbacksFound ConnectionStatus = "NoNonLoopbacksFound"
    ConnectionStatus_NoInternet     ConnectionStatus = "NoInternet"
    ConnectionStatus_Connected      ConnectionStatus = "Connected"
)

// hidden errors in net/http/transport
var (
    transport_errClosed = "net/http: transport closed before response was received"
)
// hidden errors in net/http
var (
    http_malformedPrefix = "malformed HTTP response"
)

func IsConnectionError(err error) bool {
    var basic BasicError
    if b, ok := err.(BasicError); !ok {
        basic = ExtractBasicError(err)
    } else {
        basic = b
    }
    switch basic.BasicError {
        case BasicError_CantResolveHost:
            fallthrough
        case BasicError_CantFindHost:
            fallthrough
        case BasicError_CantConnectToHost:
            return true;
    }
    return false
}

func ExtractBasicError(err error) (b BasicError) {
    log.Printf("Error: %T %#v \t\t'%s'\n", err, err, err.Error())
    b.Cause = err
    if urlErr, ok := err.(*url.Error); ok {
        if opErr, ok := urlErr.Err.(*net.OpError); ok {
            log.Printf("net.OpError: %T %#v\n", opErr, opErr)
            if opErr.Op == "dial" {
                if opErr2, ok := opErr.Err.(*net.OpError); ok {
                    log.Printf("net.OpError2: %T %#v\n", opErr2, opErr2)
                    if opErr2.Op == "ConnectEx" {
                        //opErr:  &net.OpError{Op:"dial", Net:"tcp", Addr:(*net.TCPAddr)(0xc0840b3b70), Err:(*net.OpError)(0xc08408cd00)}
                        //opErr2: &net.OpError{Op:"ConnectEx", Net:"tcp", Addr:net.Addr(nil), Err:0x274d}
                        b.BasicError = BasicError_CantConnectToHost
                        return
                    }
                }
                b.BasicError = BasicError_CantResolveHost
                return
            } else if opErr.Op == "local error" {
                b.BasicError = BasicError_CantFindHost
                return
            }
        }
    } else if opErr, ok := err.(*net.OpError); ok {
        if opErr.Op == "dial" {
            if sysError, ok := opErr.Err.(*os.SyscallError); ok {
                log.Printf("os.SyscallError: %#v\n", sysError)
                if sysError.Syscall == "GetAddrInfoW" {
                    b.BasicError = BasicError_CantResolveHost
                    return
                }
            } else if opErr2, ok := opErr.Err.(*net.OpError); ok {
                log.Printf("net.OpError %#v\n", opErr2)
                if opErr2.Op == "ConnectEx" {
                    b.BasicError = BasicError_CantConnectToHost
                    return
                }
            }
        } else if opErr.Op == "WSARecv" {
            b.BasicError = BasicError_UnexpectedEof
            return
        }
    } else if err == io.EOF {
        b.BasicError = BasicError_UnexpectedEof
        return
    } else if err.Error() == transport_errClosed {
        b.BasicError = BasicError_UnexpectedEof
        return
    } else if strings.HasPrefix(err.Error(), http_malformedPrefix) {
        b.BasicError = BasicError_BadDataReceived
        return
    }
    b.BasicError = BasicError_Unknown
    return
}

// Checks for basic internet connectivity.
// An interface is up, it has an ip. Its not a loopback. Google can be hit.
func CheckNetworkConnection() (ConnectionStatus, error) {
    return CheckNetworkConnectionAndHost("")
}

// Checks the same stuff as CheckNetworkConnection, expect will also
//  try a HEAD request to the give url
func CheckNetworkConnectionAndHost(extraUrl string) (ConnectionStatus, error) {
    interfaces, err := net.Interfaces()
    if err != nil {
        return "", err
    }
    if len(interfaces) == 0 {
        return ConnectionStatus_NoInterfaces, nil
    }
    interfacesUp := 0
    nonLoopbackAddrs := 0
    for i := range interfaces {
        if interfaces[i].Flags & net.FlagUp == 0 {
            continue
        }
        interfacesUp++

        addrs, err := interfaces[i].Addrs()
        if err != nil {
            return "", err
        }
        for k := range addrs {
            if ipAddr, ok := addrs[k].(*net.IPAddr); ok {
                if ipAddr.IP.IsLoopback() {
                    log.Println("IsLoopback")
                    continue
                }
                nonLoopbackAddrs++
            }
        }
    }
    if interfacesUp == 0 {
        return ConnectionStatus_NoInterfacesUp, nil
    }
    if nonLoopbackAddrs == 0 {
        return ConnectionStatus_NoNonLoopbacksFound, nil
    }

    var testUrls []string
    if extraUrl != "" {
        testUrls = make([]string, 2)
        testUrls[1] = extraUrl
    } else {
        testUrls = make([]string, 1)
    }
    testUrls[0] = "http://www.google.com"

    for i := range testUrls {
        _, err = http.Head(testUrls[i])
        if err != nil {
            basicErr := ExtractBasicError(err)
            if basicErr.BasicError == BasicError_CantFindHost ||
                    basicErr.BasicError == BasicError_CantResolveHost ||
                    basicErr.BasicError == BasicError_CantConnectToHost {
                return ConnectionStatus_NoInternet, nil
            } else {
                return "", err
            }
        }
    }

    return ConnectionStatus_Connected, nil
}
