Attribute VB_Name = "ImplementsProbe"
Option Explicit

' Compile alongside the original stdVBA stdICallable.cls, unchanged.
Public Function Run() As Long
    Dim callable As stdICallable, rebound As stdICallable
    Dim arguments(0 To 1) As Variant, success As Boolean
    Set callable = New ImplementsProbeWorker
    If CLng(callable.Run(20, 22)) <> 42 Then Err.Raise 5
    arguments(0) = 20
    arguments(1) = 22
    If CLng(callable.RunEx(arguments)) <> 42 Then Err.Raise 5
    Set rebound = callable.Bind(1)
    If Not rebound Is callable Then Err.Raise 5
    If CLng(callable.SendMessage("answer", success, Empty)) <> 42 Then Err.Raise 5
    If Not success Then Err.Raise 5
    Run = 42
End Function
